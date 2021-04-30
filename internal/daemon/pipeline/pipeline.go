package pipeline

import (
	"archive/zip"
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/logstashconfig"
)

type Archive struct {
	Pipelines Pipelines
	File      string
	BasePath  string
}

type Pipelines []Pipeline

type Pipeline struct {
	ID      string `yaml:"pipeline.id"`
	Config  string `yaml:"path.config"`
	Ordered string `yaml:"pipeline.ordered,omitempty"`
	Workers int    `yaml:"pipeline.workers"`

	Path     map[string]interface{} `yaml:"path,omitempty"`
	Pipeline map[string]interface{} `yaml:"pipeline,omitempty"`
}

func New(file, basePath string) (Archive, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return Archive{}, err
	}

	p := Pipelines{}
	err = yaml.Unmarshal(b, &p)
	if err != nil {
		return Archive{}, err
	}

	processNestedKeys(p)

	a := Archive{
		Pipelines: p,
		File:      file,
		BasePath:  basePath,
	}

	return a, nil
}

func processNestedKeys(pipelines Pipelines) {
	for i := range pipelines {
		if ival, ok := pipelines[i].Path["config"]; ok {
			if val, ok := ival.(string); ok {
				pipelines[i].Config = val
			}
		}

		if ival, ok := pipelines[i].Pipeline["id"]; ok {
			if val, ok := ival.(string); ok {
				pipelines[i].ID = val
			}
		}
	}
}

func (a Archive) Validate() error {
	var inputs, outputs int
	for _, pipeline := range a.Pipelines {
		files, err := doublestar.Glob(filepath.Join(a.BasePath, pipeline.Config))
		if err != nil {
			return err
		}
		for _, file := range files {
			var relFile string
			if path.IsAbs(a.BasePath) {
				relFile = strings.TrimPrefix(file, a.BasePath)
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				relFile = strings.TrimPrefix(file, filepath.Join(cwd, a.BasePath))
			}

			body, err := ioutil.ReadFile(file)
			if err != nil {
				return err
			}

			configFile := logstashconfig.File{
				Name: relFile,
				Body: body,
			}

			in, out, err := configFile.Validate()
			if err != nil {
				return err
			}
			inputs += in
			outputs += out
		}
	}

	if inputs == 0 || outputs == 0 {
		return errors.Errorf("expect the Logstash config to have at least 1 input and 1 output, got %d inputs and %d outputs", inputs, outputs)
	}

	return nil
}

func (a Archive) Zip() ([]byte, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	f, err := w.Create("pipelines.yml")
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadFile(a.File)
	if err != nil {
		return nil, err
	}
	_, err = f.Write(body)
	if err != nil {
		return nil, err
	}

	for _, pipeline := range a.Pipelines {
		files, err := doublestar.Glob(filepath.Join(a.BasePath, pipeline.Config))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			var relFile string
			if path.IsAbs(a.BasePath) {
				relFile = strings.TrimPrefix(file, a.BasePath)
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return nil, err
				}
				relFile = strings.TrimPrefix(file, filepath.Join(cwd, a.BasePath))
			}

			f, err := w.Create(relFile)
			if err != nil {
				return nil, err
			}

			body, err := ioutil.ReadFile(file)
			if err != nil {
				return nil, err
			}
			_, err = f.Write(body)
			if err != nil {
				return nil, err
			}
		}
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
