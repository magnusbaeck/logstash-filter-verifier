package logstashconfig_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matryer/is"
	"github.com/pkg/errors"

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

			is.True(file.Exists(filepath.Join(tempdir, f.Name)))                   // test.conf
			is.True(file.Contains(filepath.Join(tempdir, f.Name), string(f.Body))) // test.conf contains "test"
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
			config: "input { stdin{ id => testid } }",

			wantConfig: `input {
  pipeline {
    address => "__lfv_input_prefix_testid"
  }
}
`,
		},
		{
			name:   "successful untouched pipeline input",
			config: "input { pipeline{ id => testid } }",

			wantConfig: `input {
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

			// FIXME: Add test for the inputCodecs map as well
			_, err := f.ReplaceInputs("prefix")
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
    send_to => [
      "lfv_output_testid"
    ]
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
			config: "output { stdout{ } }",
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

func TestValidate(t *testing.T) {
	cases := []struct {
		name   string
		config string

		wantErr     error
		wantInputs  int
		wantOutputs int
	}{
		{
			name: "successful validate",
			config: `input { stdin { id => stdin } }
filter { mutate { id => mutate } }
output { stdout { id => testid } }`,

			wantInputs:  1,
			wantOutputs: 1,
		},
		{
			name: "error without ids",
			config: `input { stdin { } }
filter { mutate { } }
output { stdout { } }`,

			wantErr: errors.Errorf(`"filename.conf" no IDs found for [stdin mutate stdout]`),
		},
		{
			name: "successful validate - with pipeline input and output",
			config: `input { stdin { id => stdin } file { id => file } pipeline { id => pipeline } }
filter { mutate { id => mutate } }
output { stdout { id => testid } file { id => file } pipeline { id => pipeline } }`,

			wantInputs:  2,
			wantOutputs: 2,
		},
	}

	for _, test := range cases {
		t.Run(fmt.Sprintf("validate-%s", test.name), func(t *testing.T) {
			f := logstashconfig.File{
				Name: "filename.conf",
				Body: []byte(test.config),
			}

			inputs, outputs, err := f.Validate(false)
			compareErr(t, test.wantErr, err)
			if test.wantInputs != inputs {
				t.Errorf("expected %d inputs, got %d", test.wantInputs, inputs)
			}
			if test.wantOutputs != outputs {
				t.Errorf("expected %d outputs, got %d", test.wantOutputs, outputs)
			}
		})

		t.Run(fmt.Sprintf("validate-with-fix-%s", test.name), func(t *testing.T) {
			f := logstashconfig.File{
				Name: "filename.conf",
				Body: []byte(test.config),
			}

			_, _, err := f.Validate(true)
			if err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func compareErr(t *testing.T, wantErr, err error) {
	switch {
	case wantErr == nil && err == nil:
	case wantErr == nil && err != nil:
		t.Errorf("expect error to be nil, got: %q", err.Error())
	case wantErr != nil && err == nil:
		t.Errorf("expect error to be %q, got no error", wantErr.Error())
	case wantErr.Error() == err.Error():
	default:
		t.Errorf("expect error to be %q, got: %q", wantErr.Error(), err.Error())
	}
}
