// Plugin mocks allow to replace an existing plugin (input, output, filter),
// identified by its ID, in the config with a mock implementation.
// This comes in handy, if a filter does perform a call out to an external
// system e.g. lookup in Elasticsearch.
// An other use case is to replace pipeline input or outputs to test such
// pipelines in isolation.
// Because the existing filter is replaced with whatever is present in
// `mock`, it is also possible to remove a plugin by simple keep the
// `mock` empty (or not present).
package pluginmock

import (
	"fmt"
	"io/ioutil"

	config "github.com/breml/logstash-config"
	"github.com/breml/logstash-config/ast"
	"github.com/breml/logstash-config/ast/astutil"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Mock struct {
	// ID of the Logstash filter to be mocked.
	ID string `json:"id" yaml:"id"`

	// Logstash configuration snippet of the replacement plugin. E.g.
	//
	//     # Constant lookup, does return the same result for each event
	//     mutate {
	//       replace => {
	//         "[field]" => "value"
	//       }
	//     }
	Mock string `json:"mock,omitempty" yaml:"mock,omitempty"`
}

func FromFile(filename string) (Mocks, error) {
	if filename == "" {
		return Mocks{}, nil
	}

	mocks := []Mock{}

	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return Mocks{}, err
	}

	err = yaml.Unmarshal(body, &mocks)
	if err != nil {
		return Mocks{}, err
	}

	pluginmocks := make(Mocks, len(mocks))

	for _, m := range mocks {
		if m.Mock == "" {
			pluginmocks[m.ID] = nil
			continue
		}

		// The mock plugin definition is wrapped with filter to form a valid
		// Logstash config, which can then be parsed.
		wrappedPlugin := []byte(fmt.Sprintf("filter {\n%s\n}", m.Mock))
		cfg, err := config.Parse("", wrappedPlugin)
		if err != nil {
			return Mocks{}, err
		}

		var plugin ast.Plugin
		var recoverErr interface{}

		func() {
			defer func() {
				recoverErr = recover()
			}()
			plugin = cfg.(ast.Config).Filter[0].BranchOrPlugins[0].(ast.Plugin)
		}()

		if recoverErr != nil {
			return Mocks{}, errors.Errorf("failed to parse mock: %s", m.Mock)
		}

		pluginmocks[m.ID] = &plugin
	}

	return pluginmocks, nil
}

type Mocks map[string]*ast.Plugin

func (m Mocks) Walk(c *astutil.Cursor) {
	id, _ := c.Plugin().ID()

	if replacement, ok := m[id]; ok {
		if replacement == nil {
			c.Delete()
			return
		}
		replacement.Attributes = append(replacement.Attributes, ast.NewStringAttribute("id", id, ast.DoubleQuoted))
		c.Replace(replacement)
	}
}
