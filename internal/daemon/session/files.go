package session

const outputPipeline = `input {
  pipeline {
    address => {{ .PipelineName }}
  }
}

filter {
  mutate {
    add_tag => [ "{{ .PipelineOrigName }}_passed" ]
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
    count => 1
    codec => plain
    threads => 1
  }
}

filter {
  mutate {
    add_tag => [ "__lfv_in_passed" ]
    # Remove the fields "sequence" and "host", which are automatically created by the generator input.
    remove_field => [ "host", "sequence" ]
    # We use the message as the LFV event ID, so move this to the right field.
    replace => {
      "[__lfv_id]" => "%{[message]}"
    }
  }

  translate {
    dictionary_path => "{{ .FieldsFilename }}"
    field => "[__lfv_id]"
    destination => "[@metadata][__lfv_fields]"
    exact => true
    override => true
    # TODO: Add default value (e.g. "__lfv_fields_not_found"), if not found in dictionary
  }
  ruby {
    # TODO: If default value ("__lfv_fields_not_found"), then skip this ruby
    # code and add an tag instead
    code => 'fields = event.get("[@metadata][__lfv_fields]")
             fields.each { |key, value| event.set(key, value) }'
  }
}

output {
  pipeline {
    send_to => [__lfv_input]
  }
}
`
