package main

import (
	"net"
	"time"

	"github.com/Nitro/envoy-docker-shim/shimrpc"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	ServiceNameLabel     = "ServiceName"
	EnvironmentNameLabel = "EnvironmentName"
	DockerUrl            = "unix:///var/run/docker.sock"
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
func (p *EnvoyProxy) WithClient(fn func(c shimrpc.RegistrarClient) error) error {
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

	c := shimrpc.NewRegistrarClient(conn)
	err = fn(c)
	conn.Close()
	return err
}

// Run makes a call to the state server to register this endpoint.
func (p *EnvoyProxy) Run() {
	log.Infof("Starting up:\nFrontend: %s\nBackend: %s", p.frontendAddr, p.backendAddr)

	time.Sleep(1 * time.Second)

	container, err := ContainerForPort(DockerUrl, p.frontendAddr.Port)
	if err != nil {
		log.Fatalf("Unable to find container! (%s)", err)
	}

	envName := container.Labels[EnvironmentNameLabel]
	svcName := container.Labels[ServiceNameLabel]

	err = p.WithClient(func(c shimrpc.RegistrarClient) error {
		resp, err := c.Register(context.Background(), &shimrpc.RegistrarRequest{
			FrontendAddr:    p.frontendAddr.IP.String(),
			FrontendPort:    int32(p.frontendAddr.Port),
			BackendAddr:     p.backendAddr.IP.String(),
			BackendPort:     int32(p.backendAddr.Port),
			Action:          shimrpc.RegistrarRequest_REGISTER,
			ServiceName:     svcName,
			EnvironmentName: envName,
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
	log.Info("Shutting down!")
	err := p.WithClient(func(c shimrpc.RegistrarClient) error {
		resp, err := c.Register(context.Background(), &shimrpc.RegistrarRequest{
			FrontendAddr: p.frontendAddr.IP.String(),
			FrontendPort: int32(p.frontendAddr.Port),
			BackendAddr:  p.backendAddr.IP.String(),
			BackendPort:  int32(p.backendAddr.Port),
			Action:       shimrpc.RegistrarRequest_DEREGISTER,
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
