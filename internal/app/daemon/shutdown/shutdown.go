package shutdown

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"

	pb "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/api/grpc"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

type Shutdown struct {
	socket string

	log logging.Logger
}

func New(socket string, log logging.Logger) Shutdown {
	return Shutdown{
		socket: socket,
		log:    log,
	}
}

func (s Shutdown) Run() error {
	s.log.Debug("Shutdown on socket ", s.socket)

	conn, err := grpc.Dial(
		s.socket,
		grpc.WithInsecure(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			if d, ok := ctx.Deadline(); ok {
				return net.DialTimeout("unix", addr, time.Until(d))
			}
			return net.Dial("unix", addr)
		}))
	if err != nil {
		return err
	}
	defer conn.Close()
	c := pb.NewControlClient(conn)

	_, err = c.Shutdown(context.Background(), &pb.ShutdownRequest{})
	return err
}
