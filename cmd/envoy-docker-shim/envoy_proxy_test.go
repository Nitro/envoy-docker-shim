package main

import (
	"errors"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nitro/envoy-docker-shim/internal/envoyhttp"
	"github.com/Nitro/envoy-docker-shim/internal/shimrpc"
	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	fAddr = net.TCPAddr{
		IP:   net.ParseIP("169.254.254.1"),
		Port: 80,
	}

	bAddr = net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 31337,
	}
)

type mockDiscoveryClient struct{}

func (c *mockDiscoveryClient) ContainerFieldsForPort(port int) (*DockerSettings, error) {
	if port == 80 {
		return &DockerSettings{
			ServiceName:     "kjartan",
			EnvironmentName: "dev",
			ProxyMode:       "http",
		}, nil
	}

	return nil, errors.New("intentional mock Error!")
}

func Test_NewEnvoyProxy(t *testing.T) {
	Convey("NewEnvoyProxy()", t, func() {
		Convey("properly configures an EnvoyProxy", func() {
			proxy, err := NewEnvoyProxy(&fAddr, &bAddr, "/var/run/docker-envoy.sock")

			So(err, ShouldBeNil)
			So(proxy.ServerAddr, ShouldEqual, "/var/run/docker-envoy.sock")
			So(proxy.Discoverer, ShouldNotBeNil)
		})
	})
}

func Test_WithClient(t *testing.T) {
	Convey("WithClient()", t, func() {
		socketPath := filepath.Join(os.TempDir(), "docker-envoy.sock")
		proxy, _ := NewEnvoyProxy(&fAddr, &bAddr, socketPath)
		proxy.Retries = []int{1, 1}
		proxy.GRPCTimeout = 20 * time.Millisecond
		registrar := envoyhttp.NewRegistrar()

		Reset(func() {
			os.Remove(socketPath)
		})

		Convey("returns an error when there is no socket to connect to", func() {
			err := proxy.WithClient(func(c shimrpc.RegistrarClient) error {
				return nil
			})

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "deadline exceeded")
		})

		Convey("bubbles up errors when the server is working", func() {
			s := serveGRPC(registrar, socketPath)
			err := proxy.WithClient(func(c shimrpc.RegistrarClient) error {
				return errors.New("Fake error!")
			})

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "Fake error!")
			s.GracefulStop()
		})

		Convey("returns no errors when things are working properly", func() {
			s := serveGRPC(registrar, socketPath)
			err := proxy.WithClient(func(c shimrpc.RegistrarClient) error {
				return nil
			})

			So(err, ShouldBeNil)
			s.GracefulStop()
		})
	})
}

func Test_Run(t *testing.T) {
	Convey("Run()", t, func() {
		socketPath := filepath.Join(os.TempDir(), "docker-envoy.sock")
		proxy, _ := NewEnvoyProxy(&fAddr, &bAddr, socketPath)
		proxy.Reload = true
		proxy.Discoverer = &mockDiscoveryClient{}
		proxy.Retries = []int{1, 1}
		proxy.GRPCTimeout = 20 * time.Millisecond
		registrar := envoyhttp.NewRegistrar()

		s := serveGRPC(registrar, socketPath)

		Reset(func() {
			s.GracefulStop()
			os.Remove(socketPath)
		})

		Convey("registers with the Registrar when things are working", func() {
			So(proxy.Run, ShouldNotPanic)
		})

		Convey("panics when the Registrar is down", func() {
			s.GracefulStop()
			os.Remove(socketPath)
			So(proxy.Run, ShouldPanic)
		})
	})
}

func Test_Close(t *testing.T) {
	Convey("Close()", t, func() {
		socketPath := filepath.Join(os.TempDir(), "docker-envoy.sock")
		proxy, _ := NewEnvoyProxy(&fAddr, &bAddr, socketPath)
		proxy.Reload = true
		proxy.Discoverer = &mockDiscoveryClient{}
		proxy.Retries = []int{1, 1}
		proxy.GRPCTimeout = 20 * time.Millisecond

		registrar := envoyhttp.NewRegistrar()

		s := serveGRPC(registrar, socketPath)
		proxy.Run()

		Reset(func() {
			s.GracefulStop()
			os.Remove(socketPath)
		})

		Convey("deregisters with the Registrar when things are working", func() {
			So(proxy.Close, ShouldNotPanic)
		})
	})
}

func serveGRPC(registrar *envoyhttp.Registrar, addr string) *grpc.Server {
	lis, err := net.Listen("unix", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	shimrpc.RegisterRegistrarServer(s, registrar)

	reflection.Register(s)
	go func() {
		s.Serve(lis) // Ignore errors in this harness
	}()

	return s
}
