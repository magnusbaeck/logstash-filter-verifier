// +build tools

// This file exists only to get various parts of the toolchain
// included in go.mod.

package tools

import (
	_ "github.com/axw/gocov/gocov"
	_ "github.com/go-playground/overalls"
	_ "github.com/matm/gocov-html"
)
