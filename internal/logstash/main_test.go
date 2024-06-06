// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"
)

// If a TestMain function is present, this is executed instead of running all the tests,
// if `go test` is executed. This allows us to use the test binary as a logstashMock
// in TestParallelProcess. The logstashMock is executed, if the env var
// `TEST_MAIN=logstash-mock` is set.
func TestMain(m *testing.M) {
	switch os.Getenv("TEST_MAIN") {
	case "logstash-mock":
		logstashMock()
	default:
		os.Exit(m.Run())
	}
}

// lostashMock returns all input (unprocessed), received on the unix domain socket provided in
// the env var `TEST_SOCKET` via stdout.
func logstashMock() {
	conn, err := net.Dial("unix", os.Getenv("TEST_SOCKET"))
	if err != nil {
		log.Fatalf("Failed to dial %s with error: %s", os.Getenv("TEST_SOCKET"), err)
	}
	b, err := io.ReadAll(conn)
	if err != nil {
		log.Fatalf("Eror while reading from socket: %s", err)
	}
	fmt.Print(string(b))
}
