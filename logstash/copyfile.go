// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"io"
	"os"
)

// copyFile copies the contents of the source filename to the
// destination file path. If the destination file exists it'll be
// overwritten. Symbolic links will be followed and file mode,
// ownership, etc will not be copied.
func copyFile(sourcePath, destPath string) error {
	r, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	w, err := os.Create(destPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}
