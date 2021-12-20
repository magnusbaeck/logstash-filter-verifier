package logstashconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

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
		id = idgen.New()
	}

	var attrs []ast.Attribute
	attrs = append(attrs, ast.NewStringAttribute("address", fmt.Sprintf("%s_%s_%s", "__lfv_input", r.idPrefix, id), ast.DoubleQuoted))

	for _, attr := range c.Plugin().Attributes {
		if attr == nil {
			continue
		}
		switch attr.Name() {
		case "add_field", "tags", "type":
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
	id = pluginIDSave(id)
	outputName := fmt.Sprintf("lfv_output_%s", id)
	o.outputs = append(o.outputs, id)

	c.Replace(ast.NewPlugin("pipeline", ast.NewArrayAttribute("send_to", ast.NewStringAttribute("", outputName, ast.DoubleQuoted))))
}

func (f *File) Validate(addMissingID bool) (inputs map[string]int, outputs map[string]int, err error) {
	err = f.parse()
	if err != nil {
		return nil, nil, err
	}

	v := validator{
		pluginIDs:    map[string]int{},
		inputs:       map[string]int{},
		outputs:      map[string]int{},
		name:         pluginIDSave(f.Name),
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
		return nil, nil, errors.Errorf("%q no IDs found for %v", f.Name, v.noIDs)
	}

	for id, count := range v.pluginIDs {
		if count != 1 {
			return nil, nil, errors.Errorf("plugin id must be unique, but %q appeared %d times", id, count)
		}
	}

	if addMissingID {
		f.Body = []byte(f.config.String())
	}

	return v.inputs, v.outputs, nil
}

type validator struct {
	noIDs        []string
	pluginType   ast.PluginType
	pluginIDs    map[string]int
	inputs       map[string]int
	outputs      map[string]int
	name         string
	count        int
	addMissingID bool
}

func (v *validator) walk(c *astutil.Cursor) {
	v.count++

	name := c.Plugin().Name()

	id, err := c.Plugin().ID()
	if err != nil {
		if v.addMissingID {
			plugin := c.Plugin()
			id = fmt.Sprintf("id_missing_%s_%04d", v.name, v.count)
			plugin.Attributes = append(plugin.Attributes, ast.NewStringAttribute("id", id, ast.DoubleQuoted))

			c.Replace(plugin)
		} else {
			v.noIDs = append(v.noIDs, name)
			return
		}
	}

	v.pluginIDs[id]++
	if v.pluginType == ast.Input && name != "pipeline" {
		v.inputs[id]++
	}
	if v.pluginType == ast.Output && name != "pipeline" {
		v.outputs[id]++
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

func pluginIDSave(in string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z' ||
			r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, in)
}
