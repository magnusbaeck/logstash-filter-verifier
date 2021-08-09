package main

import (
	"os"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app"
)

// GitSummary contains "git describe" output and is automatically
// populated via linker options when building with govvv.
var GitSummary = "(unknown)"

func main() {
	exitCode := app.Execute(GitSummary, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
