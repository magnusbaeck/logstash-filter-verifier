package pipeline_test

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"

	"github.com/matryer/is"
	"gopkg.in/yaml.v2"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
)

func TestNew(t *testing.T) {
	tt := []struct {
		name   string
		config string

		want pipeline.Pipelines
	}{
		{
			name: "one pipeline",
			config: `- pipeline.id: main
  path.config: "pipelines/main/main.conf"
`,

			want: pipeline.Pipelines{
				pipeline.Pipeline{
					ID:     "main",
					Config: "pipelines/main/main.conf",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			p := pipeline.Pipelines{}

			err := yaml.Unmarshal([]byte(tc.config), &p)
			is.NoErr(err)

			is.Equal(tc.want, p)
		})
	}
}

func TestValidate(t *testing.T) {
	wd, _ := os.Getwd()

	cases := []struct {
		name     string
		pipeline string
		basePath string

		wantValidateErr bool
	}{
		{
			name:     "success basic pipeline",
			pipeline: "testdata/pipelines_basic.yml",
			basePath: "testdata/",
		},
		{
			name:     "success basic pipeline with absolute base path",
			pipeline: "testdata/pipelines_basic_base_path.yml",
			basePath: wd,
		},
		{
			name:     "error invalid config",
			pipeline: "testdata/pipelines_invalid_config.yml",
			basePath: "testdata/",

			wantValidateErr: true,
		},
		{
			name:     "success basic pipeline with nested keys",
			pipeline: "testdata/pipelines_basic_nested_keys.yml",
			basePath: "testdata/",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			a, err := pipeline.New(test.pipeline, test.basePath)
			is.NoErr(err)

			err = a.Validate()
			is.True(err != nil == test.wantValidateErr) // Validate error
		})
	}
}

func TestZip(t *testing.T) {
	wd, _ := os.Getwd()

	cases := []struct {
		name     string
		pipeline string
		basePath string

		wantNewArchiveErr bool
		wantZipBytesErr   bool
		wantFiles         int
	}{
		{
			name:     "success basic pipeline",
			pipeline: "testdata/pipelines_basic.yml",
			basePath: "testdata/",

			wantFiles: 2,
		},
		{
			name:     "success basic pipeline without base path",
			pipeline: "testdata/pipelines_basic_base_path.yml",

			wantFiles: 2,
		},
		{
			name:     "success basic pipeline with absolute base path",
			pipeline: "testdata/pipelines_basic_base_path.yml",
			basePath: wd,

			wantFiles: 2,
		},
		{
			name:     "success advanced pipeline",
			pipeline: "testdata/pipelines_advanced.yml",
			basePath: "testdata/",

			wantFiles: 3,
		},
		{
			name:     "error pipeline file not found",
			pipeline: "testdata/pipelines_invalid.yml",
			basePath: "testdata/",

			wantNewArchiveErr: true,
		},
		{
			name:     "error pipeline file not yaml",
			pipeline: "testdata/pipelines_invalid_yaml.yml",
			basePath: "testdata/",

			wantNewArchiveErr: true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			a, err := pipeline.New(test.pipeline, test.basePath)
			is.True(err != nil == test.wantNewArchiveErr) // New error

			if test.wantNewArchiveErr {
				return
			}

			b, err := a.Zip()
			is.True(err != nil == test.wantZipBytesErr) // Zip error

			if test.wantZipBytesErr {
				return
			}

			r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
			is.NoErr(err)

			is.Equal(test.wantFiles, len(r.File))
		})
	}
}
