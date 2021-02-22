package controller

import (
	"path"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
)

// TODO: With Go 1.16, this can be moved to embed files

const logstashConfig = `
pipeline.unsafe_shutdown: true
`

// TODO: Why is Logstash still printing
// Sending Logstash logs to .../3rdparty/logstash-7.10.0/logs which is now configured via log4j2.properties
// to stdout on startup?
const log4j2Config = `status = error
name = LogstashPropertiesConfig

appender.json_file.type = File
appender.json_file.name = json_file
appender.json_file.fileName = {{ .WorkDir }}/logstash.log
appender.json_file.layout.type = JSONLayout
appender.json_file.layout.compact = true
appender.json_file.layout.eventEol = true

rootLogger.level = ${sys:ls.log.level}
rootLogger.appenderRef.console.ref = json_file
`

// Dummy pipeline, which prevents logstash from stopping when all inputs have
// been finished/drained.
const stdinPipeline = `input { stdin {} }
output { stdout { } }
`

const outputPipeline = `input {
  pipeline {
    address => __lfv_output
  }
}
filter {
  mutate {
    copy => {
      "[@metadata]" => "[__metadata]"
    }
  }
  ruby {
    code => 'metadata = event.get("__metadata")
             metadata.delete_if {|key, value| key.start_with?("__lfv") || key.start_with?("__tmp") }
             if metadata.length > 0
               event.set("__metadata", metadata)
             else
               event.remove("__metadata")
             end'
  }
}
output {
  stdout {
    codec => json_lines
  }
}
`

func basePipelines(workDir string) pipeline.Pipelines {
	return pipeline.Pipelines{
		pipeline.Pipeline{
			ID:      "stdin",
			Config:  path.Join(workDir, "stdin.conf"),
			Ordered: "true",
			Workers: 1,
		},
		pipeline.Pipeline{
			ID:      "output",
			Config:  path.Join(workDir, "output.conf"),
			Ordered: "true",
			Workers: 1,
		},
	}
}
