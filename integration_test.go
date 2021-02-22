package main_test

import (
	"context"
	"os"
	"path"
	"testing"
	"time"

	"github.com/matryer/is"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/daemon"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/daemon/run"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/api/grpc"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/file"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func TestIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("integration test skipped, enable with env var `INTEGRATION_TEST=1`")
	}

	is := is.New(t)

	testLogger := &logging.LoggerMock{
		DebugFunc:    func(args ...interface{}) { t.Log(args...) },
		DebugfFunc:   func(format string, args ...interface{}) { t.Logf(format, args...) },
		ErrorFunc:    func(args ...interface{}) { t.Log(args...) },
		ErrorfFunc:   func(format string, args ...interface{}) { t.Logf(format, args...) },
		FatalFunc:    func(args ...interface{}) { t.Log(args...) },
		FatalfFunc:   func(format string, args ...interface{}) { t.Logf(format, args...) },
		InfoFunc:     func(args ...interface{}) { t.Log(args...) },
		InfofFunc:    func(format string, args ...interface{}) { t.Logf(format, args...) },
		WarningFunc:  func(args ...interface{}) { t.Log(args...) },
		WarningfFunc: func(format string, args ...interface{}) { t.Logf(format, args...) },
	}

	tempdir := t.TempDir()
	// Start Daemon
	socket := path.Join(tempdir, "integration_test.socket")
	logstashPath := path.Join("3rdparty/logstash-7.10.0/bin/logstash")
	if !file.Exists(logstashPath) {
		t.Fatalf("Logstash needs to be present in %q for the integration tests to work", logstashPath)
	}

	log := testLogger
	server := daemon.New(socket, logstashPath, log)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	go func() {
		defer cancel()

		is := is.New(t)

		defer server.Cleanup()

		err := server.Run(ctx)
		is.NoErr(err)
	}()

	i := 0
	for {
		if file.Exists(socket) {
			break
		}
		time.Sleep(100 * time.Millisecond)
		i++
		if i >= 20 {
			t.Fatalf("wait for socket file failed")
		}
	}

	// Run tests
	cases := []struct {
		name      string
		pipeline  string
		basePath  string
		testcases string
	}{
		{
			name: "basic_pipeline",
		},
		{
			name: "conditional_output",
		},
		{
			name: "pipeline_to_pipeline",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := run.New(
				path.Join(tempdir, "integration_test.socket"),
				log,
				"testdata/"+tc.name+".yml",
				"testdata/"+tc.name,
				"testdata/testcases/"+tc.name,
			)
			is.NoErr(err)

			err = client.Run()
			is.NoErr(err)
		})
	}

	_, err := server.Shutdown(context.Background(), &grpc.ShutdownRequest{})
	is.NoErr(err)

	<-ctx.Done()
}
