package session

import "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"

//go:generate moq -fmt goimports -pkg session_test -out ./logstash_controller_mock_test.go . LogstashController

type LogstashController interface {
	SetupTest(pipelines pipeline.Pipelines) error
	ExecuteTest(pipelines pipeline.Pipelines, expectedEvents int) error
	GetResults() ([]string, error)
	Teardown() error
}
