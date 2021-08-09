// Copyright (c) 2016-2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// allFilesExist returns true iff all given files exist in a
// directory, meaning if stat() is successful. Hence the files don't
// have to be regular files and a permission problem with the given
// directory that prevents stat() from succeeding will result in a
// false result.
func allFilesExist(dir string, filenames []string) bool {
	for _, filename := range filenames {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

// copyAllFiles finds the first directory in a list that contains all
// of the supplied files and copies those files to a directory. The
// directory where the files were found is returned. If no directory
// contains all the files an error is returned.
func copyAllFiles(sourceDirs []string, sourceFiles []string, dest string) (string, error) {
	for _, sourceDir := range sourceDirs {
		if !allFilesExist(sourceDir, sourceFiles) {
			continue
		}
		for _, sourceFile := range sourceFiles {
			srcFile := filepath.Join(sourceDir, sourceFile)
			destFile := filepath.Join(dest, sourceFile)
			if err := copyFile(srcFile, destFile); err != nil {
				return "", err
			}
		}
		return sourceDir, nil
	}
	return "", fmt.Errorf("couldn't find the wanted files (%s) in any of these directories: %s",
		strings.Join(sourceFiles, ", "), strings.Join(sourceDirs, ", "))
}

// copyFile copies the contents of the source filename to the
// destination file path. If the destination file exists it'll be
// overwritten. Symbolic links will be followed and file mode,
// ownership, etc will not be copied.
func copyFile(sourcePath, destPath string) error {
	r, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.Create(destPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	if err != nil {
		return err
	}
	return w.Close()
}
