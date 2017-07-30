package cmd

import (
	"testing"
	"os"
)

// these test basically just ensure there are no panics, usually b/c of mismatched tempalate and
// config variables

func TestWriteLibrarianBanner(t *testing.T) {
	WriteLibrarianBanner(os.Stdout)
}

func TestWriteAuthorBanner(t *testing.T) {
	WriteAuthorBanner(os.Stdout)
}
