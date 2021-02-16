package logstashconfig

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	config "github.com/breml/logstash-config"
	"github.com/breml/logstash-config/ast"
	"github.com/breml/logstash-config/ast/astutil"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/idgen"
)

type File struct {
	Name string
	Body []byte

	config *ast.Config
}

func (f File) Save(targetDir string) error {
	err := os.MkdirAll(path.Join(targetDir, path.Dir(f.Name)), 0700)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path.Join(targetDir, f.Name), f.Body, 0600)
	if err != nil {
		return err
	}

	return nil
}

func (f *File) parse() error {
	if f.config != nil {
		return nil
	}

	icfg, err := config.Parse(f.Name, f.Body)
	if err != nil {
		return err
	}

	cfg, ok := icfg.(ast.Config)
	if !ok {
		return errors.New("not a valid logstash config")
	}

	f.config = &cfg

	return nil
}

func (f *File) ReplaceInputs() error {
	err := f.parse()
	if err != nil {
		return err
	}

	for i := range f.config.Input {
		astutil.ApplyPlugins(f.config.Input[i].BranchOrPlugins, replaceInputs)
	}

	f.Body = []byte(f.config.String())

	return nil
}

func replaceInputs(c *astutil.Cursor) {
	if c.Plugin() == nil || c.Plugin().Name() == "pipeline" {
		return
	}

	// TODO: __lfv_input must reflect the actual input, that has been replaced, such that this input
	// can be referenced in the test case configuration.
	c.Replace(ast.NewPlugin("pipeline", ast.NewStringAttribute("address", "__lfv_input", ast.Bareword)))
}

func (f *File) ReplaceOutputs() ([]string, error) {
	err := f.parse()
	if err != nil {
		return nil, err
	}

	outputs := outputPipelineReplacer{
		outputs: make([]string, 0),
	}
	for i := range f.config.Output {
		astutil.ApplyPlugins(f.config.Output[i].BranchOrPlugins, outputs.walk)
	}

	f.Body = []byte(f.config.String())

	return outputs.outputs, nil
}

type outputPipelineReplacer struct {
	outputs []string
}

func (o *outputPipelineReplacer) walk(c *astutil.Cursor) {
	if c.Plugin() == nil || c.Plugin().Name() == "pipeline" {
		return
	}

	id, err := c.Plugin().ID()
	if err != nil {
		id = idgen.New()
	}
	outputName := fmt.Sprintf("lfv_output_%s", id)
	o.outputs = append(o.outputs, id)

	c.Replace(ast.NewPlugin("pipeline", ast.NewArrayAttribute("send_to", ast.NewStringAttribute("", outputName, ast.DoubleQuoted))))
}
