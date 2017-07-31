package cmd

import (
	"io"
	"path"
	"runtime"
	"text/template"
	"time"

	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/version"
)

const (
	librarianTemplateFilename = "librarian.template.txt"
	authorTemplateFilename    = "author.template.txt"
	bannersDir                = "banners"
)

type librarianConfig struct {
	LibriVersion string
	Now          string
	GoVersion    string
	GoOS         string
	GoArch       string
	NumCPU       int
}

type authorConfig struct {
	LibriVersion string
}

func WriteLibrarianBanner(w io.Writer) {
	config := &librarianConfig{
		LibriVersion: version.Version.String(),
		Now:          time.Now().UTC().Format(time.RFC3339),
		GoVersion:    runtime.Version(),
		GoOS:         runtime.GOOS,
		GoArch:       runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
	}
	relPath := path.Join(bannersDir, librarianTemplateFilename)
	tmpl, err := template.New(librarianTemplateFilename).ParseFiles(relPath)
	errors.MaybePanic(err)
	err = tmpl.Execute(w, config)
	errors.MaybePanic(err)
}

func WriteAuthorBanner(w io.Writer) {
	config := &authorConfig{
		LibriVersion: version.Version.String(),
	}
	relPath := path.Join(bannersDir, authorTemplateFilename)
	tmpl, err := template.New(authorTemplateFilename).ParseFiles(relPath)
	errors.MaybePanic(err)
	err = tmpl.Execute(w, config)
	errors.MaybePanic(err)
}
