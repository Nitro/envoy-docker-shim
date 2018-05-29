package main

import (
	"fmt"
	"net"
	"time"

	"log"

	"github.com/Nitro/envoy-docker-shim/envoyrpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type EnvoyProxy struct {
	ServerAddr   string
	frontendAddr *net.TCPAddr
	backendAddr  *net.TCPAddr
}

func NewEnvoyProxy(frontendAddr, backendAddr net.Addr, svrAddr string) (*EnvoyProxy, error) {

	front := frontendAddr.(*net.TCPAddr)
	back := backendAddr.(*net.TCPAddr)

	return &EnvoyProxy{
		frontendAddr: front,
		backendAddr:  back,
		ServerAddr:   svrAddr,
	}, nil
}

// WithClient is a wrapper to make a new connection and close it with each call.
// We should have extremely low throughput so this provides a level of safety by
// reconnection each time.
func (p *EnvoyProxy) WithClient(fn func(c envoyrpc.RegistrarClient) error) error {
	conn, err := grpc.Dial(p.ServerAddr,
		grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			log.Printf("Connecting on Unix socket: %s", addr)
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	c := envoyrpc.NewRegistrarClient(conn)
	err = fn(c)
	conn.Close()
	return err
}

// Run makes a call to the state server to register this endpoint.
func (p *EnvoyProxy) Run() {
	fmt.Printf("Starting up:\nFrontend: %s\bBackend: %s\n", p.frontendAddr, p.backendAddr)
	err := p.WithClient(func(c envoyrpc.RegistrarClient) error {
		resp, err := c.Register(context.Background(), &envoyrpc.RegistrarRequest{
			FrontendAddr: p.frontendAddr.IP.String(),
			FrontendPort: int32(p.frontendAddr.Port),
			BackendAddr:  p.backendAddr.IP.String(),
			BackendPort:  int32(p.backendAddr.Port),
			Action:       envoyrpc.RegistrarRequest_REGISTER,
		})
		if err == nil {
			log.Printf("Status: %v", resp.StatusCode)
		}
		return err
	})

	if err != nil {
		log.Fatalf("Could not call Envoy: %s", err)
	}

	// Wait for the signal handler to shut us down
	select {}
}

// Close makes a call to the state server to shut down this endpoint.
func (p *EnvoyProxy) Close() {
	fmt.Printf("Shutting down!")
	err := p.WithClient(func(c envoyrpc.RegistrarClient) error {
		resp, err := c.Register(context.Background(), &envoyrpc.RegistrarRequest{
			FrontendAddr: p.frontendAddr.IP.String(),
			FrontendPort: int32(p.frontendAddr.Port),
			BackendAddr:  p.backendAddr.IP.String(),
			BackendPort:  int32(p.backendAddr.Port),
			Action:       envoyrpc.RegistrarRequest_DEREGISTER,
		})
		if err == nil {
			log.Printf("Status: %v", resp.StatusCode)
		}
		return err
	})

	if err != nil {
		log.Fatalf("Could not call Envoy: %s", err)
	}
}

// FrontendAddr returns the frontend address.
func (p *EnvoyProxy) FrontendAddr() net.Addr { return p.frontendAddr }

// BackendAddr returns the backend address.
func (p *EnvoyProxy) BackendAddr() net.Addr { return p.backendAddr }
