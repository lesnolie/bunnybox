// Package client connects the GUI process directly to the official worker pipes.
// It intentionally has no dependency on generated service stubs.
package client

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"singboxwin.local/client/internal/rpc/worker"
)

type Config struct {
	// Language returns the current GUI BCP47 language tag. Default: en.
	Language    func() string
	DialTimeout time.Duration
}

type Client struct {
	mu                  sync.Mutex
	manager             *worker.Manager
	config              Config
	process             *worker.Process
	application, daemon *grpc.ClientConn
	closed              bool
	dial                func(context.Context, string) (net.Conn, error)
}

func New(manager *worker.Manager, config Config) *Client {
	if config.Language == nil {
		config.Language = func() string { return "en" }
	}
	if config.DialTimeout <= 0 {
		config.DialTimeout = 5 * time.Second
	}
	return &Client{manager: manager, config: config, dial: dialPipe}
}

func (c *Client) Application(ctx context.Context) (*grpc.ClientConn, error) {
	return c.connection(ctx, true)
}
func (c *Client) Daemon(ctx context.Context) (*grpc.ClientConn, error) {
	return c.connection(ctx, false)
}

func (c *Client) connection(ctx context.Context, application bool) (*grpc.ClientConn, error) {
	// The caller must also give each RPC an appropriate deadline. A successful
	// transport dial is not proof of daemon ownership or a started core service.
	p, err := c.manager.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, worker.ErrClosed
	}
	if c.process != p {
		c.closeConnections()
		c.process = p
	}
	pipe, authority, existing := p.DaemonPipe, "sing-box", c.daemon
	if application {
		pipe, authority, existing = p.ApplicationPipe, "sing-box-worker", c.application
	}
	if existing != nil {
		return existing, nil
	}
	conn, err := c.dialTransport(ctx, p, pipe, authority)
	if err != nil {
		return nil, err
	}
	if application {
		c.application = conn
	} else {
		c.daemon = conn
	}
	return conn, nil
}

func (c *Client) dialTransport(ctx context.Context, p *worker.Process, pipe, authority string) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, c.config.DialTimeout)
	defer cancel()
	// DialContext with WithBlock is intentional: NewClient returns before any
	// connection is attempted and must not be exposed as a connected transport.
	conn, err := grpc.DialContext(dialCtx, "passthrough:///"+authority,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithAuthority(authority),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return c.dial(ctx, pipe) }),
		grpc.WithBlock(),
		grpc.WithChainUnaryInterceptor(c.unary(p)),
		grpc.WithChainStreamInterceptor(c.stream(p)),
	)
	if err != nil {
		return nil, replaceExit(p, err)
	}
	select {
	case <-p.Done():
		conn.Close()
		return nil, p.ExitError()
	default:
	}
	return conn, nil
}

func (c *Client) language(ctx context.Context) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	lang := c.config.Language()
	if lang == "" {
		lang = "en"
	}
	md.Set("accept-language", lang)
	return metadata.NewOutgoingContext(ctx, md)
}

func replaceExit(p *worker.Process, err error) error {
	if err == nil {
		return nil
	}
	select {
	case <-p.Done():
		return p.ExitError()
	default:
		return err
	}
}

func (c *Client) unary(p *worker.Process) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoke grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return replaceExit(p, invoke(c.language(ctx), method, req, reply, cc, opts...))
	}
}

func (c *Client) stream(p *worker.Process) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		s, err := streamer(c.language(ctx), desc, cc, method, opts...)
		if err != nil {
			return nil, replaceExit(p, err)
		}
		return &exitStream{ClientStream: s, process: p}, nil
	}
}

type exitStream struct {
	grpc.ClientStream
	process *worker.Process
}

func (s *exitStream) RecvMsg(m any) error { return replaceExit(s.process, s.ClientStream.RecvMsg(m)) }
func (s *exitStream) SendMsg(m any) error { return replaceExit(s.process, s.ClientStream.SendMsg(m)) }
func (s *exitStream) Header() (metadata.MD, error) {
	md, err := s.ClientStream.Header()
	return md, replaceExit(s.process, err)
}
func (s *exitStream) CloseSend() error { return replaceExit(s.process, s.ClientStream.CloseSend()) }

func (c *Client) closeConnections() error {
	var errs []error
	if c.application != nil {
		errs = append(errs, c.application.Close())
		c.application = nil
	}
	if c.daemon != nil {
		errs = append(errs, c.daemon.Close())
		c.daemon = nil
	}
	return errors.Join(errs...)
}

// Close closes transports before stdin EOF and waits for the worker to exit.
func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	err := c.closeConnections()
	c.mu.Unlock()
	return errors.Join(err, c.manager.Close())
}
