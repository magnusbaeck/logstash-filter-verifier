package logstashconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/pkg/errors"

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

	return ioutil.WriteFile(path.Join(targetDir, f.Name), f.Body, 0600)
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

func (f *File) ReplaceInputs(idPrefix string) (map[string]string, error) {
	err := f.parse()
	if err != nil {
		return nil, err
	}

	w := replaceInputsWalker{
		idPrefix:    idPrefix,
		inputCodecs: map[string]string{},
	}

	for i := range f.config.Input {
		astutil.ApplyPlugins(f.config.Input[i].BranchOrPlugins, w.replaceInputs)
	}

	f.Body = []byte(f.config.String())

	return w.inputCodecs, nil
}

type replaceInputsWalker struct {
	idPrefix    string
	inputCodecs map[string]string
}

func (r replaceInputsWalker) replaceInputs(c *astutil.Cursor) {
	if c.Plugin() == nil || c.Plugin().Name() == "pipeline" {
		return
	}

	id, err := c.Plugin().ID()
	if err != nil {
		panic(err)
	}

	var attrs []ast.Attribute
	attrs = append(attrs, ast.NewStringAttribute("address", fmt.Sprintf("%s_%s_%s", "__lfv_input", r.idPrefix, id), ast.Bareword))

	for _, attr := range c.Plugin().Attributes {
		if attr == nil {
			continue
		}
		switch attr.Name() {
		case "add_field", "tags":
			attrs = append(attrs, attr)
		case "codec":
			r.inputCodecs[id] = attr.String()
		default:
		}
	}

	// TODO: __lfv_input must reflect the actual input, that has been replaced, such that this input
	// can be referenced in the test case configuration.
	c.Replace(ast.NewPlugin("pipeline", attrs...))
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

func (f *File) Validate() (inputs int, outputs int, err error) {
	err = f.parse()
	if err != nil {
		return 0, 0, err
	}

	v := validator{}

	for i := range f.config.Input {
		v.pluginType = ast.Input
		astutil.ApplyPlugins(f.config.Input[i].BranchOrPlugins, v.walk)
	}
	for i := range f.config.Filter {
		v.pluginType = ast.Filter
		astutil.ApplyPlugins(f.config.Filter[i].BranchOrPlugins, v.walk)
	}
	for i := range f.config.Output {
		v.pluginType = ast.Output
		astutil.ApplyPlugins(f.config.Output[i].BranchOrPlugins, v.walk)
	}

	if len(v.noIDs) > 0 {
		return 0, 0, errors.Errorf("%q no IDs found for %v", f.Name, v.noIDs)
	}
	return v.inputs, v.outputs, nil
}

type validator struct {
	noIDs      []string
	pluginType ast.PluginType
	inputs     int
	outputs    int
}

func (v *validator) walk(c *astutil.Cursor) {
	_, err := c.Plugin().ID()
	if err != nil {
		v.noIDs = append(v.noIDs, c.Plugin().Name())
	}

	if v.pluginType == ast.Input && c.Plugin().Name() != "pipeline" {
		v.inputs++
	}
	if v.pluginType == ast.Output && c.Plugin().Name() != "pipeline" {
		v.outputs++
	}
}
