package idgen

import (
	nanoid "github.com/matoous/go-nanoid"
)

const (
	alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	size     = 8
)

func New() string {
	return nanoid.MustGenerate(alphabet, size)
}
