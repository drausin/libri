package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/drausin/libri/libri/common/errors"
)

const (
	templateFilepath = "dashboard.service.json.template"
	outputFilepath   = "dashboard.service.json"
)

// Config defines the values used in the service template.
type Config struct {
	Endpoints []string
}

var config = Config{
	Endpoints: []string{"Get", "Put", "Find", "Store", "Verify"},
}

func main() {
	wd, err := os.Getwd()
	errors.MaybePanic(err)
	absTemplateFilepath := filepath.Join(wd, templateFilepath)

	funcs := template.FuncMap{
		"timesplus": func(x, y, z int) int { return x*y + z },
	}
	tmpl, err := template.New(templateFilepath).
		Funcs(funcs).
		Delims("[[", "]]").
		ParseFiles(absTemplateFilepath)
	errors.MaybePanic(err)

	absOutFilepath := filepath.Join(wd, outputFilepath)
	out, err := os.Create(absOutFilepath)
	errors.MaybePanic(err)
	err = tmpl.Execute(out, config)
	errors.MaybePanic(err)
	fmt.Printf("wrote %s\n", outputFilepath)
}
