package template

import (
	"bytes"
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
	err = os.WriteFile(filename, b.Bytes(), perm)
	if err != nil {
		return err
	}
	return nil
}
