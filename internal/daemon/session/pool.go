package session

import "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pool"

//go:generate moq -fmt goimports -pkg session_test -out ./pool_mock_test.go . Pool

type Pool interface {
	Get() (pool.LogstashController, error)
	Return(instance pool.LogstashController, clean bool)
}
