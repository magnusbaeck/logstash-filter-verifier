package session

import (
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pool"
)

//go:generate moq -fmt goimports -pkg session_test -out ./logstash_controller_mock_test.go . LogstashController

type LogstashController = pool.LogstashController
