// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	testData := "random string\n"

	// Create the source file.
	source, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("Unexpected error when creating file: %s", err)
	}
	defer os.Remove(source.Name())
	source.Write([]byte(testData))

	// Create the destination directory.
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Unexpected error when creating temp dir: %s", err)
	}
	defer os.RemoveAll(tempdir)
	destPath := filepath.Join(tempdir, "arbitrary-filename")

	err = copyFile(source.Name(), destPath)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	buf, err := ioutil.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Unexpected error reading destination file: %s", err)
	}
	actualData := string(buf)
	if actualData != testData {
		t.Fatalf("Destination file contained %q after copying, expected %q.", actualData, testData)
	}
}
