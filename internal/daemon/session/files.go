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
input {
  file {
    id => '__lfv_file_input'
    path => "{{ .InputFilename }}"
    {{ .InputCodec }}
    mode => "read"
    file_completed_action => "log"
    file_completed_log_path => "{{ .InputFilename }}.log"
    exit_after_read => true
    delimiter => "xyTY1zS2mwJ9xuFCIkrPucLtiSuYIkXAmgCXB142"
  }
}

filter {
  ruby {
    id => '__lfv_ruby_count'
    init => '@count = 0'
    code => 'event.set("[@metadata][__lfv_id]", @count.to_s)
             @count += 1'
    tag_on_exception => '__lfv_ruby_count_exception'
  }

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
