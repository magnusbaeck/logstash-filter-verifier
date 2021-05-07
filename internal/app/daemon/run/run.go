package run

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path"
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

	pb "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/api/grpc"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/filtermock"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logstash"
	lfvobserver "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/observer"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/testcase"
)

type Test struct {
	socket       string
	pipeline     string
	pipelineBase string
	testcasePath string
	filterMock   string
	metadataKey  string
	debug        bool

	log logging.Logger
}

func New(socket string, log logging.Logger, pipeline, pipelineBase, testcasePath, filterMock, metadataKey string, debug bool) (Test, error) {
	if !path.IsAbs(pipelineBase) {
		cwd, err := os.Getwd()
		if err != nil {
			return Test{}, err
		}
		pipelineBase = filepath.Join(cwd, pipelineBase)
	}
	return Test{
		socket:       socket,
		pipeline:     pipeline,
		pipelineBase: pipelineBase,
		testcasePath: testcasePath,
		filterMock:   filterMock,
		metadataKey:  metadataKey,
		debug:        debug,
		log:          log,
	}, nil
}

func (s Test) Run() error {
	a, err := pipeline.New(s.pipeline, s.pipelineBase)
	if err != nil {
		return err
	}

	// TODO: ensure, that IDs are also unique for the whole set of pipelines
	err = a.Validate()
	if err != nil {
		return err
	}

	m, err := filtermock.FromFile(s.filterMock)
	if err != nil {
		return err
	}

	preprocessor := pipeline.NoopPreprocessor
	if len(m) > 0 {
		preprocessor = pipeline.ApplyMocksPreprocessor(m)
	}

	b, err := a.ZipWithPreprocessor(preprocessor)
	if err != nil {
		return err
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

	tests, err := testcase.DiscoverTests(s.testcasePath)
	if err != nil {
		return err
	}

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
			SessionID:   sessionID,
			InputPlugin: t.InputPlugin,
			InputLines:  t.InputLines,
			Events:      b,
		})
		if err != nil {
			return err
		}

		results, err := s.postProcessResults(result.Results, t.ExportMetadata)
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

	_, err = c.TeardownTest(context.Background(), &pb.TeardownTestRequest{
		SessionID: sessionID,
		Stats:     false,
	})
	if err != nil {
		return err
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

func (s Test) validateInputLines(lines []string) {
	for _, line := range lines {
		_, doubleQuoteErr := astutil.Quote(line, ast.DoubleQuoted)
		_, singleQuoteErr := astutil.Quote(line, ast.SingleQuoted)

		if doubleQuoteErr != nil && singleQuoteErr != nil {
			s.log.Warningf("Test input line %q contains unescaped double and single quotes, single quotes will be escaped automatically", line)
		}
	}
}

func (s Test) postProcessResults(results []string, exportMetadata bool) ([]string, error) {
	var err error

	sort.Slice(results, func(i, j int) bool {
		return gjson.Get(results[i], `__lfv_id`).Int() < gjson.Get(results[j], `__lfv_id`).Int()
	})

	for i := range results {
		if exportMetadata {
			metadata := gjson.Get(results[i], "__lfv_metadata")
			if metadata.Exists() && metadata.IsObject() {
				md := make(map[string]json.RawMessage, len(metadata.Map()))
				for key, value := range metadata.Map() {
					if strings.HasPrefix(key, "__lfv") || strings.HasPrefix(key, "__tmp") {
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

		results[i], err = sjson.Delete(results[i], "__lfv_id")
		if err != nil {
			return nil, err
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
