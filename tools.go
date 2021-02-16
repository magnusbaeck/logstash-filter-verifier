// +build tools

// This file exists only to get various parts of the toolchain
// included in go.mod.

package tools

import (
	_ "github.com/ahmetb/govvv"
	_ "github.com/axw/gocov/gocov"
	_ "github.com/go-playground/overalls"
	_ "github.com/matm/gocov-html"
	_ "github.com/matryer/moq"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
