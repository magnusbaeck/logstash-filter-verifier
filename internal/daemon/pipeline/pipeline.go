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
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pluginmock"
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

func (a Archive) ZipWithPreprocessor(addMissingID bool, preprocess func([]byte) ([]byte, error)) (data []byte, inputs map[string]int, err error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	f, err := w.Create("pipelines.yml")
	if err != nil {
		return nil, nil, err
	}
	body, err := ioutil.ReadFile(a.File)
	if err != nil {
		return nil, nil, err
	}
	_, err = f.Write(body)
	if err != nil {
		return nil, nil, err
	}

	inputs = map[string]int{}
	outputs := map[string]int{}
	for _, pipeline := range a.Pipelines {
		if strings.HasSuffix(pipeline.Config, "/") {
			pipeline.Config += "*"
		}
		configFilepath := filepath.Join(a.BasePath, pipeline.Config)
		if filepath.IsAbs(pipeline.Config) {
			configFilepath = pipeline.Config
		}
		files, err := doublestar.Glob(configFilepath)
		if err != nil {
			return nil, nil, err
		}
		for _, file := range files {
			fi, err := os.Stat(file)
			if err != nil {
				return nil, nil, err
			}
			if fi.IsDir() {
				continue
			}
			var relFile string
			if path.IsAbs(a.BasePath) {
				relFile = strings.TrimPrefix(file, a.BasePath)
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return nil, nil, err
				}
				relFile = strings.TrimPrefix(file, filepath.Join(cwd, a.BasePath))
			}

			f, err := w.Create(relFile)
			if err != nil {
				return nil, nil, err
			}

			body, err := ioutil.ReadFile(file)
			if err != nil {
				return nil, nil, err
			}

			body, err = preprocess(body)
			if err != nil {
				return nil, nil, err
			}

			configFile := logstashconfig.File{
				Name: relFile,
				Body: body,
			}

			in, out, err := configFile.Validate(addMissingID)
			if err != nil {
				return nil, nil, err
			}

			for id, count := range in {
				inputs[id] += count
			}
			for id, count := range out {
				outputs[id] += count
			}

			_, err = f.Write(configFile.Body)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	err = w.Close()
	if err != nil {
		return nil, nil, err
	}

	if len(inputs) == 0 || len(outputs) == 0 {
		return nil, nil, errors.Errorf("expect the Logstash config to have at least 1 input and 1 output, got %d inputs and %d outputs", len(inputs), len(outputs))
	}

	return buf.Bytes(), inputs, nil
}

func NoopPreprocessor(body []byte) ([]byte, error) {
	return body, nil
}

func ApplyMocksPreprocessor(m pluginmock.Mocks) func(body []byte) ([]byte, error) {
	return func(body []byte) ([]byte, error) {
		configFile := logstashconfig.File{
			Body: body,
		}

		err := configFile.ApplyMocks(m)
		if err != nil {
			return body, err
		}

		return configFile.Body, nil
	}
}
