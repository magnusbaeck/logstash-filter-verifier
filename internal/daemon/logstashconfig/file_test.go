package logstashconfig_test

import (
	"path"
	"strings"
	"testing"

	"github.com/matryer/is"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/file"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/logstashconfig"
)

func TestSave(t *testing.T) {
	cases := []struct {
		name string
	}{
		{
			name: "successful",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			tempdir := t.TempDir()

			f := logstashconfig.File{
				Name: "test.conf",
				Body: []byte("test"),
			}

			err := f.Save(tempdir)
			is.NoErr(err)

			is.True(file.Exists(path.Join(tempdir, f.Name)))                   // test.conf
			is.True(file.Contains(path.Join(tempdir, f.Name), string(f.Body))) // test.conf contains "test"
		})
	}
}

func TestReplaceInputs(t *testing.T) {
	cases := []struct {
		name   string
		config string

		wantConfig string
	}{
		{
			name:   "successful replacement",
			config: "input { stdin{} }",

			wantConfig: `input {
  pipeline {
    address => __lfv_input
  }
}
`,
		},
		{
			name:   "successful untouched pipeline input",
			config: "input { pipeline{} }",

			wantConfig: `input {
  pipeline {}
}
`,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			f := logstashconfig.File{
				Body: []byte(test.config),
			}

			err := f.ReplaceInputs()
			is.NoErr(err)

			is.Equal(test.wantConfig, string(f.Body))
		})
	}
}

func TestReplaceOutputs(t *testing.T) {
	cases := []struct {
		name   string
		config string

		wantOutputs []string
		wantConfig  string
	}{
		{
			name:   "successful replace",
			config: "output { stdout{ id => testid } }",

			wantOutputs: []string{"testid"},
			wantConfig: `output {
  pipeline {
    send_to => [ "lfv_output_testid" ]
  }
}
`,
		},
		{
			name:   "successful untouched pipeline output",
			config: "output { pipeline{ id => testid } }",

			wantOutputs: []string{},
			wantConfig: `output {
  pipeline {
    id => testid
  }
}
`,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			f := logstashconfig.File{
				Body: []byte(test.config),
			}

			outputs, err := f.ReplaceOutputs()
			is.NoErr(err)

			is.Equal(test.wantOutputs, outputs)
			is.Equal(test.wantConfig, string(f.Body))
		})
	}
}

func TestReplaceOutputsWithoutID(t *testing.T) {
	cases := []struct {
		name   string
		config string

		wantOutputs []string
		wantConfig  string
	}{
		{
			name:   "successful replacement without id",
			config: "output { stdout{} }",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			f := logstashconfig.File{
				Body: []byte(test.config),
			}

			outputs, err := f.ReplaceOutputs()
			is.NoErr(err)

			is.True(len(outputs) == 1)                               // len(outputs) == 1
			is.True(strings.Contains(string(f.Body), "pipeline"))    // f.Body contains "pipeline"
			is.True(strings.Contains(string(f.Body), "lfv_output_")) // f.Body contains "lfv_output_"
		})
	}
}
