package controller_test

import (
	"errors"
	"path"
	"testing"

	"github.com/matryer/is"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/controller"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/file"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/instance/mock"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func TestNewController(t *testing.T) {
	cases := []struct {
		name string
	}{
		{
			name: "success",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			tempdir := t.TempDir()

			c, err := controller.NewController(nil, tempdir, logging.NoopLogger, nil)
			is.NoErr(err)

			is.True(file.Exists(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "logstash.yml")))      // logstash.yml
			is.True(file.Exists(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "log4j2.properties"))) // log4j2.properties
			is.True(file.Exists(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "stdin.conf")))        // stdin.conf
			is.True(file.Exists(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "output.conf")))       // output.conf
			is.True(file.Exists(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "pipelines.yml")))     // pipelines.yml
		})
	}
}

func TestLaunch(t *testing.T) {
	cases := []struct {
		name             string
		instanceStartErr error

		wantErr bool
	}{
		{
			name: "success",
		},
		{
			name:             "instance start error",
			instanceStartErr: errors.New("error"),

			wantErr: true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			instance := &mock.InstanceMock{
				StartFunc: func(controllerMoqParam *controller.Controller, workdir string) error {
					return test.instanceStartErr
				},
				ShutdownFunc: func() {},
			}

			tempdir := t.TempDir()

			c, err := controller.NewController(instance, tempdir, logging.NoopLogger, nil)
			is.NoErr(err)

			err = c.Launch()
			is.True(err != nil == test.wantErr) // Launch error
		})
	}
}

func TestCompleteCycle(t *testing.T) {
	cases := []struct {
		name string
	}{
		{
			name: "success",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			instance := &mock.InstanceMock{
				StartFunc: func(controllerMoqParam *controller.Controller, workdir string) error {
					return nil
				},
				ConfigReloadFunc: func() error {
					return nil
				},
			}

			tempdir := t.TempDir()

			shutdown := make(chan struct{})

			c, err := controller.NewController(instance, tempdir, logging.NoopLogger, shutdown)
			is.NoErr(err)

			err = c.Launch()
			is.NoErr(err)

			// Simulate pipelines ready from instance
			c.PipelinesReady("stdin", "output")

			pipelines := pipeline.Pipelines{
				pipeline.Pipeline{
					ID:      "main",
					Config:  "main.conf",
					Ordered: "true",
					Workers: 1,
				},
			}

			err = c.SetupTest(pipelines)
			is.NoErr(err)

			// Simulate pipelines ready from instance
			c.PipelinesReady("stdin", "output", "main")

			pipelines = append(pipelines, pipeline.Pipeline{
				ID:      "input",
				Config:  "input.conf",
				Ordered: "true",
				Workers: 1,
			})

			err = c.ExecuteTest(pipelines, 2)
			is.NoErr(err)

			// Simulate pipelines ready from instance
			c.PipelinesReady("stdin", "output", "main", "input")
			err = c.ReceiveEvent("result 1")
			is.NoErr(err)
			err = c.ReceiveEvent("result 2")
			is.NoErr(err)

			res, err := c.GetResults()
			is.NoErr(err)
			is.Equal(2, len(res))

			// Test content of pipeline.yml
			is.True(file.Exists(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "pipelines.yml")))                // pipelines.yml
			is.True(file.Contains(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "pipelines.yml"), "id: main"))  // pipelines.yml contains "id: main"
			is.True(file.Contains(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "pipelines.yml"), "id: input")) // pipelines.yml contains "id: input"

			err = c.Teardown()
			is.NoErr(err)

			// Test if pipelines are reomved from pipeline.yml
			is.True(file.Exists(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "pipelines.yml")))                 // pipelines.yml
			is.True(!file.Contains(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "pipelines.yml"), "id: main"))  // pipelines.yml contains "id: main"
			is.True(!file.Contains(path.Join(tempdir, controller.LogstashInstanceDirectoryPrefix, c.ID(), "pipelines.yml"), "id: input")) // pipelines.yml contains "id: input"
		})
	}
}

func TestSetupTest_Shutdown(t *testing.T) {
	cases := []struct {
		name string
	}{
		{
			name: "success",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			instance := &mock.InstanceMock{
				StartFunc: func(controllerMoqParam *controller.Controller, workdir string) error {
					return nil
				},
				ConfigReloadFunc: func() error {
					return nil
				},
			}

			tempdir := t.TempDir()
			shutdown := make(chan struct{})

			c, err := controller.NewController(instance, tempdir, logging.NoopLogger, shutdown)
			is.NoErr(err)

			err = c.Launch()
			is.NoErr(err)

			// Simulate shutdown signal
			close(shutdown)

			pipelines := pipeline.Pipelines{
				pipeline.Pipeline{
					ID:      "main",
					Config:  "main.conf",
					Ordered: "true",
					Workers: 1,
				},
			}

			err = c.SetupTest(pipelines)
			is.True(err != nil) // expect shutdown error
		})
	}
}
