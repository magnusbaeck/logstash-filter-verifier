package session

const outputPipeline = `input {
  pipeline {
    address => {{ .PipelineName }}
  }
}

filter {
  mutate {
    add_tag => [ "__lfv_out_{{ .PipelineOrigName }}_passed" ]
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
  generator {
    lines => [
      {{ .InputLines }}
    ]
    {{ .InputCodec }}
    count => 1
    threads => 1
  }
}

filter {
  ruby {
    id => '__lfv_ruby_count'
    init => '@count = 0'
    code => 'event.set("__lfv_id", @count.to_s)
             @count += 1'
    tag_on_exception => '__lfv_ruby_count_exception'
  }

  mutate {
    add_tag => [ "__lfv_in_passed" ]
    # Remove fields "host", "sequence" and optionally "message", which are
    # automatically created by the generator input.
    remove_field => [ {{ .RemoveGeneratorFields }} ]
  }

  translate {
    dictionary_path => "{{ .FieldsFilename }}"
    field => "[__lfv_id]"
    destination => "[@metadata][__lfv_fields]"
    exact => true
    override => true
    fallback => "__lfv_fields_not_found"
    refresh_interval => 0
  }

  ruby {
    id => '__lfv_ruby_fields'
    code => 'fields = event.get("[@metadata][__lfv_fields]")
             fields.each { |key, value| event.set(key, value) } unless fields == "__lfv_fields_not_found"
             event.tag("lfv_fields_not_found") if fields == "__lfv_fields_not_found"'
    tag_on_exception => '__lfv_ruby_fields_exception'
  }
}

output {
  pipeline {
    send_to => [ "{{ .InputPluginName }}" ]
  }
}
`
