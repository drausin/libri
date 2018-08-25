package cmd

import (
	"io"
	"runtime"
	"text/template"
	"time"

	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/version"
)

const librarianTemplate = `

   ██╗       ██╗   ██████╗    ██████╗    ██╗
   ██║       ██║   ██╔══██╗   ██╔══██╗   ██║
   ██║       ██║   ██████╔╝   ██████╔╝   ██║
   ██║       ██║   ██╔══██╗   ██╔══██╗   ██║
   ███████╗  ██║   ██████╔╝   ██║  ██║   ██║
   ╚══════╝  ╚═╝   ╚═════╝    ╚═╝  ╚═╝   ╚═╝

Libri Librarian Server

Libri version   {{ .LibriVersion }}
Build Date:     {{ .BuildDate }}
Git Branch:     {{ .GitBranch }}
Git Revision:   {{ .GitRevision }}
Go version:     {{ .GoVersion }}
GOOS:           {{ .GoOS }}
GOARCH:         {{ .GoArch }}
NumCPU:         {{ .NumCPU }}

`

const authorTemplate = `Libri Author Client v{{ .LibriVersion }}
`

type librarianConfig struct {
	LibriVersion string
	GitBranch    string
	GitRevision  string
	BuildDate    string
	Now          string
	GoVersion    string
	GoOS         string
	GoArch       string
	NumCPU       int
}

type authorConfig struct {
	LibriVersion string
}

// WriteLibrarianBanner writes the librarian banner to the io.Writer.
func WriteLibrarianBanner(w io.Writer) {
	config := &librarianConfig{
		LibriVersion: version.Current.Version.String(),
		GitBranch:    version.Current.GitBranch,
		GitRevision:  version.Current.GitRevision,
		BuildDate:    version.Current.BuildDate,
		Now:          time.Now().UTC().Format(time.RFC3339),
		GoVersion:    runtime.Version(),
		GoOS:         runtime.GOOS,
		GoArch:       runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
	}
	tmpl, err := template.New("librarian-banner").Parse(librarianTemplate)
	errors.MaybePanic(err)
	err = tmpl.Execute(w, config)
	errors.MaybePanic(err)
	time.Sleep(10 * time.Millisecond)
}

// WriteAuthorBanner writes the author banner to the io.Writer.
func WriteAuthorBanner(w io.Writer) {
	config := &authorConfig{
		LibriVersion: version.Current.Version.String(),
	}
	tmpl, err := template.New("author-banner").Parse(authorTemplate)
	errors.MaybePanic(err)
	err = tmpl.Execute(w, config)
	errors.MaybePanic(err)
}
