package logstash

import (
	"context"
	"io"
)

// stdinBlockReader implements the io.Reader interface and blocks reading
// until the shutdown channel unblocks (close of channel).
// After the shutdown channel is unblocked, the Read function
// returns io.EOF to signal the end of the input stream
//
// This stdinBlockReader is used to block stdin of the controlled
// Logstash instance.
type stdinBlockReader struct {
	ctx context.Context
}

func (s *stdinBlockReader) Read(_ []byte) (int, error) {
	<-s.ctx.Done()
	return 0, io.EOF
}
