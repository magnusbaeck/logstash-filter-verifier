// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package testhelpers

import (
	"os"
	"path/filepath"
)

// FileWithMode contains information about a pathname and its desired
// filemode and contents. It can be used to quickly create those files
// in tests that rely on files in the file system.
type FileWithMode struct {
	Path     string
	Mode     os.FileMode
	Contents string
}

// Create creates the regular file or directory described by the
// FileWithMode type instance.
func (fwp FileWithMode) Create(dir string) error {
	path := filepath.Join(dir, fwp.Path)
	if fwp.Mode&os.ModeDir != 0 {
		return os.Mkdir(path, fwp.Mode&os.ModePerm)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(fwp.Contents)
	if err != nil {
		return err
	}
	return f.Chmod(fwp.Mode & os.ModePerm)
}
