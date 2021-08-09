package template

import (
	"bytes"
	"io/ioutil"
	"os"
	"text/template"
)

func ToFile(filename string, templateContent string, data interface{}, perm os.FileMode) error {
	tmpl, err := template.New("template").Parse(templateContent)
	if err != nil {
		return err
	}
	b := &bytes.Buffer{}
	err = tmpl.Execute(b, data)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, b.Bytes(), perm)
	if err != nil {
		return err
	}
	return nil
}
