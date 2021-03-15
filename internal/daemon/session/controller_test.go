package session_test

import (
	"path"
	"testing"
	"time"

	"github.com/matryer/is"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/file"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/logstashconfig"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pool"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/session"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func TestSession(t *testing.T) {
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

			pool := &PoolMock{
				GetFunc: func() (pool.LogstashController, error) {
					logstashController := &LogstashControllerMock{
						SetupTestFunc: func(pipelines pipeline.Pipelines) error {
							is.True(len(pipelines) == 2) // Expect 2 pipelines (main, output)
							return nil
						},
						TeardownFunc: func() error {
							return nil
						},
						ExecuteTestFunc: func(pipelines pipeline.Pipelines, expectedEvents int) error {
							is.True(len(pipelines) == 3) // Expect 2 pipelines (input, main, output)
							return nil
						},
						GetResultsFunc: func() ([]string, error) {
							return []string{"some_random_result"}, nil
						},
					}
					return logstashController, nil
				},
				ReturnFunc: func(instance pool.LogstashController, clean bool) {},
			}

			c := session.NewController(tempdir, pool, logging.NoopLogger)

			pipelines := pipeline.Pipelines{
				pipeline.Pipeline{
					ID:      "main",
					Config:  "main.conf",
					Ordered: "true",
					Workers: 1,
				},
			}

			configFiles := []logstashconfig.File{
				{
					Name: "main.conf",
					Body: []byte(`input { stdin{ id => testid } } filter { mutate{ add_tag => [ "test" ] } } output { stdout{} }`),
				},
			}

			s, err := c.Create(pipelines, configFiles)
			is.NoErr(err)

			is.True(file.Exists(path.Join(tempdir, "session", s.ID(), "sut", "main.conf")))                  // sut/main.conf
			is.True(file.Contains(path.Join(tempdir, "session", s.ID(), "sut", "main.conf"), "__lfv_input")) // sut/main.conf contains "__lfv_input"
			is.True(file.Contains(path.Join(tempdir, "session", s.ID(), "sut", "main.conf"), "lfv_output_")) // sut/main.conf contains "lfv_output_"

			_, err = c.Get("invalid")
			is.True(err != nil) // Get invalid session error

			s, err = c.Get(s.ID())
			is.NoErr(err)

			inputLines := []string{"some_random_input"}
			inFields := []map[string]interface{}{
				{
					"some_random_key": "value",
				},
			}
			err = s.ExecuteTest("input", inputLines, inFields)
			is.NoErr(err)

			is.True(file.Exists(path.Join(tempdir, "session", s.ID(), "lfv_inputs", "1", "fields.json")))                              // lfv_inputs/1/fields.json
			is.True(file.Contains(path.Join(tempdir, "session", s.ID(), "lfv_inputs", "1", "fields.json"), "some_random_key"))         // lfv_inputs/1/fields.json contains "some_random_key"
			is.True(file.Exists(path.Join(tempdir, "session", s.ID(), "lfv_inputs", "1", "input.conf")))                               // lfv_inputs/1/input.conf
			is.True(file.Contains(path.Join(tempdir, "session", s.ID(), "lfv_inputs", "1", "input.conf"), "lfv_inputs/1/fields.json")) // lfv_inputs/1/input.conf contains "lfv_inputs/1/fields.json"

			results, err := s.GetResults()
			is.NoErr(err)
			is.True(len(results) > 0) // GetResults does return results

			err = c.DestroyByID("invalid")
			is.True(err != nil) // DestroyByID invalid session error

			err = c.DestroyByID(s.ID())
			is.NoErr(err)

			<-c.WaitFinish()
		})
	}
}

func TestCreate(t *testing.T) {
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

			pool := &PoolMock{
				GetFunc: func() (pool.LogstashController, error) {
					logstashController := &LogstashControllerMock{
						SetupTestFunc: func(pipelines pipeline.Pipelines) error {
							return nil
						},
						TeardownFunc: func() error {
							return nil
						},
					}
					return logstashController, nil
				},
				ReturnFunc: func(instance pool.LogstashController, clean bool) {},
			}

			c := session.NewController(tempdir, pool, logging.NoopLogger)

			pipelines := pipeline.Pipelines{
				pipeline.Pipeline{
					ID:      "main",
					Config:  "main.conf",
					Ordered: "true",
					Workers: 1,
				},
			}

			configFiles := []logstashconfig.File{
				{
					Name: "main.conf",
					Body: []byte(`input { stdin{ id => testid } } filter { mutate{ add_tag => [ "test" ] } } output { stdout{} }`),
				},
			}

			s, err := c.Create(pipelines, configFiles)
			is.NoErr(err)

			go func() {
				time.Sleep(10 * time.Millisecond)
				err = c.DestroyByID(s.ID())
				is.NoErr(err)
			}()

			s2, err := c.Create(pipelines, configFiles)
			is.NoErr(err)

			is.True(s.ID() != s2.ID()) // IDs of two separate sessions are not equal
		})
	}
}
