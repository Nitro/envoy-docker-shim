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
	// Labels looked up in Docker to identify the service and environment names
	// as well as the proxy mode.
	ServiceNameLabel     = "ServiceName"
	EnvironmentNameLabel = "EnvironmentName"
	ProxyModeLabel       = "ProxyMode"
)

// An EnvoyProxy is a proxy instance that using a shim service to configure
// and maintain an instance of Lyft's Envoy proxy on the host in place
// of a normal docker-proxy instance.
type EnvoyProxy struct {
	ServerAddr   string
	frontendAddr *net.TCPAddr
	backendAddr  *net.TCPAddr
	Discoverer   DiscoveryClient
	Reload       bool // Are we waiting around or just reloading the settings?
}

// NewEnvoyProxy returns a correctly configured EnvoyProxy.
func NewEnvoyProxy(frontendAddr, backendAddr net.Addr, svrAddr string) (*EnvoyProxy, error) {
	front := frontendAddr.(*net.TCPAddr)
	back := backendAddr.(*net.TCPAddr)

	return &EnvoyProxy{
		frontendAddr: front,
		backendAddr:  back,
		ServerAddr:   svrAddr,
		Discoverer:   &DockerClient{},
	}, nil
}

// WithClient is a wrapper to make a new connection and close it with each call.
// We should have extremely low throughput so this provides a level of safety by
// reconnection each time.
func (p *EnvoyProxy) WithClient(fn func(c shimrpc.RegistrarClient) error) error {
	conn, err := grpc.Dial(p.ServerAddr,
		grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			log.Infof("Connecting on Unix socket: %s", addr)
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

// DoAction wrpas up all the right settings into a request and sticks in
// the requested action. It then calls the GRPC server using the client
// returned from WithClient.
func (p *EnvoyProxy) DoAction(action shimrpc.RegistrarRequest_Action) error {
	settings := p.Discoverer.ContainerFieldsForPort(p.frontendAddr.Port)
	req := p.RequestWithSettings(settings)
	req.Action = action

	return p.WithClient(func(c shimrpc.RegistrarClient) error {
		resp, err := c.Register(context.Background(), req)
		if err == nil {
			log.Infof("Status: %v", resp.StatusCode)
		}
		return err
	})
}

// withRetries is a decorator to retry with fixed durations
func withRetries(retries []int, fn func() error) error {
	var err error
	for _, millis := range retries {
		err = fn()
		if err == nil {
			return nil
		}

		time.Sleep(time.Duration(millis) * time.Millisecond)
	}

	return err
}

// Run makes a call to the state server to register this endpoint.
func (p *EnvoyProxy) Run() {
	log.Infof("Starting up:\nFrontend: %s\nBackend: %s", p.frontendAddr, p.backendAddr)

	// Have to give Docker a quick breather to see the container.
	// XXX maybe watch events or poll the API instead?
	time.Sleep(1 * time.Second)

	err := withRetries([]int{100, 500, 1000, 1500}, func() error {
		err2 := p.DoAction(shimrpc.RegistrarRequest_REGISTER)
		if err2 != nil {
			log.Warn("Retrying...")
		}
		return err2
	})

	if err != nil {
		log.Fatalf("Could not call Envoy: %s", err)
	}

	// Wait for the signal handler to shut us down
	if !p.Reload {
		select {}
	}
}

// Close makes a call to the state server to shut down this endpoint.
func (p *EnvoyProxy) Close() {
	log.Info("Shutting down!")

	err := withRetries([]int{100, 500, 1000, 1500}, func() error {
		return p.DoAction(shimrpc.RegistrarRequest_DEREGISTER)
	})

	if err != nil {
		log.Fatalf("Could not call Envoy: %s", err)
	}
}

// RequestWithSettings returns a properly formatted shimrpc Request
// using the DockerSettings passed in.
func (p *EnvoyProxy) RequestWithSettings(settings *DockerSettings) *shimrpc.RegistrarRequest {
	return &shimrpc.RegistrarRequest{
		FrontendAddr:    p.frontendAddr.IP.String(),
		FrontendPort:    int32(p.frontendAddr.Port),
		BackendAddr:     p.backendAddr.IP.String(),
		BackendPort:     int32(p.backendAddr.Port),
		ServiceName:     settings.ServiceName,
		EnvironmentName: settings.EnvironmentName,
		ProxyMode:       settings.ProxyMode,
	}
}

// FrontendAddr returns the frontend address.
func (p *EnvoyProxy) FrontendAddr() net.Addr { return p.frontendAddr }

// BackendAddr returns the backend address.
func (p *EnvoyProxy) BackendAddr() net.Addr { return p.backendAddr }
