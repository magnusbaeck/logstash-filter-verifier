package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app"
)

func buildSetting(key string) (string, bool) {
	info, _ := debug.ReadBuildInfo()
	for _, kv := range info.Settings {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return "", false
}

func version() string {
	var vcsinfo = []string{}
	if vcs, found := buildSetting("vcs"); found {
		vcsinfo = append(vcsinfo, vcs)
	}
	if revision, found := buildSetting("vcs.revision"); found {
		if len(revision) > 7 {
			revision = revision[0:7]
		}
		vcsinfo = append(vcsinfo, revision)
	}
	if time, found := buildSetting("vcs.time"); found {
		vcsinfo = append(vcsinfo, time)
	}
	if modified, found := buildSetting("vcs.modified"); found {
		if modified == "true" {
			vcsinfo = append(vcsinfo, "dirty")
		}
	}
	var version = "(unknown)"
	if len(vcsinfo) > 0 {
		version = strings.Join(vcsinfo, " ")
	}
	return version
}

func main() {
	exitCode := app.Execute(version(), os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
