//go:build debug

// Package dbg provides DD: spew to stderr then exit 1 (works with Recover).
// Build with -tags debug to enable (e.g. go run -tags debug ./cmd/api).
package dbg

import (
	"os"

	"github.com/davecgh/go-spew/spew"
)

func DD(v ...any) {
	for _, x := range v {
		spew.Fdump(os.Stderr, x)
	}
	os.Exit(1)
}
