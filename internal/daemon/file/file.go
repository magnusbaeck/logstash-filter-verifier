package file

import (
	"bytes"
	"os"
)

func Exists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func Contains(filename string, needle string) bool {
	body, err := os.ReadFile(filename)
	if err != nil {
		return false
	}
	return bytes.Contains(body, []byte(needle))
}
