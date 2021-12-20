package session

const outputPipeline = `input {
  pipeline {
    address => {{ .PipelineName }}
  }
}

filter {
  mutate {
    add_field => { "[@metadata][__lfv_out_passed]" => "{{ .PipelineOrigName }}" }
  }
}

output {
  pipeline {
    send_to => __lfv_output
  }
}
`

const inputGenerator = `
{{ $root := . }}
{{ range $i, $index := .InputIndices }}
input {
  file {
    id => '__lfv_file_input_{{ $index }}'
    path => "{{ $root.InputFilename }}_{{ $index }}"
    {{ $root.InputCodec }}
    mode => "read"
    file_completed_action => "log"
    file_completed_log_path => "{{ $root.InputFilename }}_{{ $index }}.log"
    exit_after_read => true
    delimiter => "xyTY1zS2mwJ9xuFCIkrPucLtiSuYIkXAmgCXB142"
    add_field => {
      "[@metadata][__lfv_id]" => "{{ $i }}"
    }
  }
}
{{ end }}

filter {
  mutate {
    # Remove fields "host", "path", "[@metadata][host]" and "[@metadata][path]"
    # which are automatically created by the file input.
    remove_field => [ "host", "path", "[@metadata][host]", "[@metadata][path]" ]
  }

{{ if .HasFields }}
  translate {
    dictionary_path => "{{ .FieldsFilename }}"
    field => "[@metadata][__lfv_id]"
    destination => "[@metadata][__lfv_fields]"
    exact => true
    override => true
    fallback => "__lfv_fields_not_found"
    refresh_interval => 0
  }
{{ end }}

  ruby {
    id => '__lfv_ruby_fields'
    code => 'fields = event.get("[@metadata][__lfv_fields]")
             fields.each { |key, value| event.set(key, value) } unless fields == "__lfv_fields_not_found"
             event.tag("lfv_fields_not_found") if fields == "__lfv_fields_not_found"
             event.remove("[message]") if event.get("[message]") == "{{ .DummyEventInputIndicator }}"'
    tag_on_exception => '__lfv_ruby_fields_exception'
  }
}

output {
  pipeline {
    send_to => [ "{{ .InputPluginName }}" ]
  }
}
`
