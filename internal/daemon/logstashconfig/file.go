package logstashconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	config "github.com/breml/logstash-config"
	"github.com/breml/logstash-config/ast"
	"github.com/breml/logstash-config/ast/astutil"
	"github.com/pkg/errors"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/idgen"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pluginmock"
)

type File struct {
	Name string
	Body []byte

	config *ast.Config
}

func (f File) Save(targetDir string) error {
	err := os.MkdirAll(filepath.Join(targetDir, path.Dir(f.Name)), 0700)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(targetDir, f.Name), f.Body, 0600)
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
	attrs = append(attrs, ast.NewStringAttribute("address", fmt.Sprintf("%s_%s_%s", "__lfv_input", r.idPrefix, id), ast.DoubleQuoted))

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

func (f *File) Validate(addMissingID bool) (inputs int, outputs int, err error) {
	err = f.parse()
	if err != nil {
		return 0, 0, err
	}

	v := validator{
		addMissingID: addMissingID,
	}

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
	noIDs        []string
	pluginType   ast.PluginType
	inputs       int
	outputs      int
	count        int
	addMissingID bool
}

func (v *validator) walk(c *astutil.Cursor) {
	v.count++

	if v.pluginType == ast.Input && c.Plugin().Name() != "pipeline" {
		v.inputs++
	}
	if v.pluginType == ast.Output && c.Plugin().Name() != "pipeline" {
		v.outputs++
	}

	_, err := c.Plugin().ID()
	if err != nil {
		if v.addMissingID {
			plugin := c.Plugin()
			plugin.Attributes = append(plugin.Attributes, ast.NewStringAttribute("id", fmt.Sprintf("%s-%d", c.Plugin().Name(), v.count), ast.DoubleQuoted))

			c.Replace(plugin)
		} else {
			v.noIDs = append(v.noIDs, c.Plugin().Name())
		}
	}
}

func (f *File) ApplyMocks(m pluginmock.Mocks) error {
	err := f.parse()
	if err != nil {
		return err
	}

	for i := 0; i < len(f.config.Input); i++ {
		f.config.Input[i].BranchOrPlugins = astutil.ApplyPlugins(f.config.Input[i].BranchOrPlugins, m.Walk)
	}
	for i := 0; i < len(f.config.Filter); i++ {
		f.config.Filter[i].BranchOrPlugins = astutil.ApplyPlugins(f.config.Filter[i].BranchOrPlugins, m.Walk)
	}
	for i := 0; i < len(f.config.Output); i++ {
		f.config.Output[i].BranchOrPlugins = astutil.ApplyPlugins(f.config.Output[i].BranchOrPlugins, m.Walk)
	}

	f.Body = []byte(f.config.String())

	return nil
}
