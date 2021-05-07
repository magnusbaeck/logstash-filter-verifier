// Filter mocks allow to replace an existing filter, identified by its ID, in
// the config with a mock implementation.
// This comes in handy, if a filter does perform a call out to an external
// system e.g. lookup in Elasticsearch.
// Because the existing filter is replaced with whatever is present in
// `filter`, it is also possible to remove a filter by simple keep the
// `filter` empty (or not present).
package filtermock

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

	// Logstash configuration snippet of the replacement filter. E.g.
	//
	//     # Constant lookup, does return the same result for each event
	//     mutate {
	//       replace => {
	//         "[field]" => "value"
	//       }
	//     }
	Filter string `json:"filter,omitempty" yaml:"filter,omitempty"`
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

	filtermocks := make(Mocks, len(mocks))

	for _, m := range mocks {
		if m.Filter == "" {
			filtermocks[m.ID] = nil
			continue
		}

		wrappedFilter := []byte(fmt.Sprintf("filter {\n%s\n}", m.Filter))
		cfg, err := config.Parse("", wrappedFilter)
		if err != nil {
			return Mocks{}, err
		}

		var filter ast.Plugin
		var recoverErr interface{}

		func() {
			defer func() {
				recoverErr = recover()
			}()
			filter = cfg.(ast.Config).Filter[0].BranchOrPlugins[0].(ast.Plugin)
		}()

		if recoverErr != nil {
			return Mocks{}, errors.Errorf("failed to parse mock filter: %s", m.Filter)
		}

		filtermocks[m.ID] = &filter
	}

	return filtermocks, nil
}

type Mocks map[string]*ast.Plugin

func (m Mocks) Walk(c *astutil.Cursor) {
	id, _ := c.Plugin().ID()

	if replacement, ok := m[id]; ok {
		if replacement == nil {
			c.Delete()
			return
		}
		replacement.Attributes = append(replacement.Attributes, ast.NewStringAttribute("id", id, ast.Bareword))
		c.Replace(replacement)
	}
}
