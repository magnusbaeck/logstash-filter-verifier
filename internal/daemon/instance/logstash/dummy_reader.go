package logstash

import "io"

// stdinBlockReader implements the io.Reader interface and blocks reading
// until the shutdown channel unblocks (close of channel).
// After the shutdown channel is unblocked, the Read function
// returns io.EOF to signal the end of the input stream
//
// This stdinBlockReader is used to block stdin of the controlled
// Logstash instance.
type stdinBlockReader struct {
	shutdown chan struct{}
}

func (s *stdinBlockReader) Read(_ []byte) (int, error) {
	<-s.shutdown
	return 0, io.EOF
}
