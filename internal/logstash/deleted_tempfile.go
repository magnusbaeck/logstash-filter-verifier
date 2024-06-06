// Copyright (c) 2015-2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"os"
)

// deletedTempFile acts like a regular file (by embedding os.File) but
// deletes the file when the file is closed.
type deletedTempFile struct {
	*os.File
}

// NewDeletedTempFile creates a new temporary file that will be deleted
// upon closing. It uses os.CreateTemp for the creation of the file and
// the dir and prefix parameters are passed straight through.
func newDeletedTempFile(dir, prefix string) (*deletedTempFile, error) {
	f, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return nil, err
	}
	return &deletedTempFile{f}, nil
}

// Close closes the underlying file and deletes it. The deletion will
// still take place even if the file closing fails but the error
// returned will be the one returned from the call to the Close method
// of the embedded os.File.
func (f *deletedTempFile) Close() error {
	closeErr := f.File.Close()
	removeErr := os.Remove(f.Name())
	if closeErr != nil {
		return closeErr
	}
	if removeErr != nil {
		log.Errorf("Problem deleting temporary file: %s", removeErr.Error())
	}
	return nil
}
