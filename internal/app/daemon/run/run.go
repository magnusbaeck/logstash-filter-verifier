package run

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/breml/logstash-config/ast"
	"github.com/breml/logstash-config/ast/astutil"
	"github.com/imkira/go-observer"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	pb "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/api/grpc"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pluginmock"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logstash"
	lfvobserver "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/observer"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/testcase"
)

type Test struct {
	socket         string
	pipeline       string
	pipelineBase   string
	logstashConfig string
	testcasePath   string
	pluginMock     string
	metadataKey    string
	debug          bool
	addMissingID   bool

	log logging.Logger
}

func New(socket string, log logging.Logger, pipeline, pipelineBase, logstashConfig, testcasePath, pluginMock, metadataKey string, debug, addMissingID bool) (Test, error) {
	if pipelineBase == "" {
		absPipeline, err := filepath.Abs(pipeline)
		if err != nil {
			return Test{}, err
		}
		pipelineBase = filepath.Dir(absPipeline)
	}
	if !filepath.IsAbs(pipelineBase) {
		cwd, err := os.Getwd()
		if err != nil {
			return Test{}, err
		}
		pipelineBase = filepath.Join(cwd, pipelineBase)
	}
	return Test{
		socket:         socket,
		pipeline:       pipeline,
		pipelineBase:   pipelineBase,
		logstashConfig: logstashConfig,
		testcasePath:   testcasePath,
		pluginMock:     pluginMock,
		metadataKey:    metadataKey,
		debug:          debug,
		addMissingID:   addMissingID,
		log:            log,
	}, nil
}

func (s Test) Run() (err error) {
	if s.logstashConfig != "" {
		pipelineFile, err := s.createImplicitPipeline()
		if err != nil {
			return err
		}
		defer os.RemoveAll(filepath.Dir(pipelineFile))

		s.pipeline = pipelineFile
		s.pipelineBase = ""
	}

	a, err := pipeline.New(s.pipeline, s.pipelineBase)
	if err != nil {
		return err
	}

	m, err := pluginmock.FromFile(s.pluginMock)
	if err != nil {
		return err
	}

	preprocessor := pipeline.NoopPreprocessor
	if len(m) > 0 {
		preprocessor = pipeline.ApplyMocksPreprocessor(m)
	}

	// TODO: ensure, that IDs are also unique for the whole set of pipelines
	b, inputs, err := a.ZipWithPreprocessor(s.addMissingID, preprocessor)
	if err != nil {
		return err
	}

	tests, err := testcase.DiscoverTests(s.testcasePath)
	if err != nil {
		return err
	}
	for _, test := range tests {
		if _, ok := inputs[test.InputPlugin]; !ok {
			return errors.Errorf("input plugin %q defined in test case but not present in Logstash config", test.InputPlugin)
		}
	}

	s.log.Debugf("socket to daemon %q", s.socket)
	conn, err := grpc.Dial(
		s.socket,
		grpc.WithInsecure(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			if d, ok := ctx.Deadline(); ok {
				return net.DialTimeout("unix", addr, time.Until(d))
			}
			return net.Dial("unix", addr)
		}))
	if err != nil {
		return err
	}
	defer conn.Close()
	c := pb.NewControlClient(conn)

	result, err := c.SetupTest(context.Background(), &pb.SetupTestRequest{
		Pipeline: b,
	})
	if err != nil {
		return err
	}
	sessionID := result.SessionID

	defer func() {
		_, teardownErr := c.TeardownTest(context.Background(), &pb.TeardownTestRequest{
			SessionID: sessionID,
			Stats:     false,
		})
		if teardownErr != nil {
			err = fmt.Errorf("failed to teardown connection: %v, root cause: %v", teardownErr, err)
		}
	}()

	observers := make([]lfvobserver.Interface, 0)
	liveObserver := observer.NewProperty(lfvobserver.TestExecutionStart{})
	observers = append(observers, lfvobserver.NewSummaryObserver(liveObserver))
	for _, obs := range observers {
		if err := obs.Start(); err != nil {
			return err
		}
	}

	testsPassed := true
	for _, t := range tests {
		b, err := json.Marshal(t.Events)
		if err != nil {
			return err
		}
		s.validateInputLines(t.InputLines)

		result, err := c.ExecuteTest(context.Background(), &pb.ExecuteTestRequest{
			SessionID:      sessionID,
			InputPlugin:    t.InputPlugin,
			InputLines:     t.InputLines,
			Events:         b,
			ExpectedEvents: int32(len(t.ExpectedEvents)),
		})
		if err != nil {
			return err
		}

		results, err := s.postProcessResults(result.Results, t)
		if err != nil {
			return err
		}

		var events []logstash.Event
		for _, line := range results {
			var event logstash.Event
			err = json.Unmarshal([]byte(line), &event)
			if err != nil {
				return err
			}
			events = append(events, event)
		}

		ok, err := t.Compare(events, []string{"diff", "-u"}, liveObserver)
		if err != nil {
			return err
		}
		if !ok {
			testsPassed = false
		}
	}

	liveObserver.Update(lfvobserver.TestExecutionEnd{})

	for _, obs := range observers {
		if err := obs.Finalize(); err != nil {
			return err
		}
	}

	if !testsPassed {
		return errors.New("failed test cases")
	}

	return nil
}

func (s Test) createImplicitPipeline() (string, error) {
	fi, err := os.Stat(s.logstashConfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to read logstash config")
	}

	pipelineBaseDir, err := os.MkdirTemp("", "lfv-pipeline-*")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary directory for implicit pipeline")
	}

	if fi.IsDir() {
		s.logstashConfig = filepath.Join(s.logstashConfig, "*")
	}

	pipelines := pipeline.Pipelines{
		pipeline.Pipeline{
			ID:      "lfv_implicit",
			Config:  s.logstashConfig,
			Workers: 1,
		},
	}

	body, err := yaml.Marshal(pipelines)
	if err != nil {
		return "", err
	}

	pipelineFile := filepath.Join(pipelineBaseDir, "pipelines.yml")
	if err := os.WriteFile(pipelineFile, body, 0600); err != nil {
		return "", errors.Wrap(err, "failed to write implicit pipelines.yml file")
	}

	return pipelineFile, nil
}

func (s Test) validateInputLines(lines []string) {
	for _, line := range lines {
		_, doubleQuoteErr := astutil.Quote(line, ast.DoubleQuoted)
		_, singleQuoteErr := astutil.Quote(line, ast.SingleQuoted)

		if doubleQuoteErr != nil && singleQuoteErr != nil {
			s.log.Warningf("Test input line %q contains unescaped double and single quotes, single quotes will be escaped automatically", line)
		}
	}
}

func (s Test) postProcessResults(results []string, t testcase.TestCaseSet) ([]string, error) {
	var err error

	sort.Slice(results, func(i, j int) bool {
		idI := gjson.Get(results[i], `__lfv_metadata.__lfv_id`).Int()
		idJ := gjson.Get(results[j], `__lfv_metadata.__lfv_id`).Int()
		if idI == idJ {
			return gjson.Get(results[i], `__lfv_metadata.__lfv_out_passed`).String() < gjson.Get(results[j], `__lfv_metadata.__lfv_out_passed`).String()
		}
		return idI < idJ
	})

	for i := 0; i < len(results); i++ {
		if s.debug {
			results[i], err = sjson.Set(results[i], `__lfv_id`, gjson.Get(results[i], `__lfv_metadata.__lfv_id`).String())
			if err != nil {
				return nil, err
			}
		}

		if t.ExportOutputs || s.debug {
			results[i], err = sjson.Set(results[i], `__lfv_out_passed`, gjson.Get(results[i], `__lfv_metadata.__lfv_out_passed`).String())
			if err != nil {
				return nil, err
			}
		}

		// Export metadata
		if t.ExportMetadata {
			metadata := gjson.Get(results[i], "__lfv_metadata")
			if metadata.Exists() && metadata.IsObject() {
				md := make(map[string]json.RawMessage, len(metadata.Map()))
				for key, value := range metadata.Map() {
					if strings.HasPrefix(key, "__lfv_") || strings.HasPrefix(key, "__tmp_") {
						continue
					}
					md[key] = json.RawMessage(value.Raw)
				}
				if len(md) > 0 {
					results[i], err = sjson.Set(results[i], s.metadataKey, md)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		results[i], err = sjson.Delete(results[i], "__lfv_metadata")
		if err != nil {
			return nil, err
		}

		// No cleanup if debug is set
		if s.debug {
			continue
		}

		tags := []string{}
		for _, tag := range gjson.Get(results[i], "tags").Array() {
			if strings.HasPrefix(tag.String(), "__lfv_") {
				continue
			}
			tags = append(tags, tag.String())
		}

		// Remove tag entry, if there are no tags
		if len(tags) == 0 {
			results[i], err = sjson.Delete(results[i], "tags")
		} else {
			results[i], err = sjson.Set(results[i], "tags", tags)
		}
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}
