package logstash

import (
	"testing"

	"github.com/matryer/is"
)

func TestExtractPipelines(t *testing.T) {
	cases := []struct {
		name  string
		input string

		want []string
	}{
		{
			name:  "single word pipeline id",
			input: `[:output, :stdin, :lfv_output_stdout, :lfv_ukPSsPZk_main, :lfv_input_1]`,

			want: []string{"output", "stdin", "lfv_output_stdout", "lfv_ukPSsPZk_main", "lfv_input_1"},
		},
		{
			name:  "pipeline id with dash",
			input: `[:output, :stdin, :lfv_output_stdout, :"lfv_ukPSsPZk_main-test", :lfv_input_1]`,

			want: []string{"output", "stdin", "lfv_output_stdout", "lfv_ukPSsPZk_main-test", "lfv_input_1"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			got := extractPipelines(test.input)
			is.Equal(test.want, got)
		})
	}
}
