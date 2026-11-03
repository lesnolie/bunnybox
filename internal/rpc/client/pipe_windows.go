package client

import (
	"context"
	"github.com/Microsoft/go-winio"
	"net"
)

func dialPipe(ctx context.Context, pipe string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, pipe)
}
