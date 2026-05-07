//go:build !debug

// Package dbg provides DD as a no-op unless built with -tags debug.
package dbg

// DD is a no-op in non-debug builds so release binaries carry no dump/exit path.
func DD(v ...any) {
	_ = v
}
