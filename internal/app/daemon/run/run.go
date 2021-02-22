package run

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path"
	"time"

	"github.com/imkira/go-observer"
	"google.golang.org/grpc"

	"github.com/magnusbaeck/logstash-filter-verifier/logstash"
	lfvobserver "github.com/magnusbaeck/logstash-filter-verifier/observer"
	"github.com/magnusbaeck/logstash-filter-verifier/testcase"
	pb "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/api/grpc"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

type Test struct {
	socket       string
	pipeline     string
	pipelineBase string
	testcasePath string

	log logging.Logger
}

func New(socket string, log logging.Logger, pipeline, pipelineBase, testcasePath string) (Test, error) {
	if !path.IsAbs(pipelineBase) {
		cwd, err := os.Getwd()
		if err != nil {
			return Test{}, err
		}
		pipelineBase = path.Join(cwd, pipelineBase)
	}
	return Test{
		socket:       socket,
		pipeline:     pipeline,
		pipelineBase: pipelineBase,
		testcasePath: testcasePath,
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

	b, err := a.Zip()
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

	for _, t := range tests {
		b, err := json.Marshal(t.InputFields)
		if err != nil {
			return err
		}
		result, err := c.ExecuteTest(context.Background(), &pb.ExecuteTestRequest{
			SessionID:  sessionID,
			InputLines: t.InputLines,
			Fields:     b,
		})
		if err != nil {
			return err
		}

		var events []logstash.Event
		for _, line := range result.Results {
			var event logstash.Event
			err = json.Unmarshal([]byte(line), &event)
			if err != nil {
				return err
			}
			events = append(events, event)
		}

		_, err = t.Compare(events, []string{"diff", "-u"}, liveObserver)
		if err != nil {
			return err
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

	return nil
}
