package main

// This file derives from libnetwork from Docker, Inc.
// It is licensed under the Apache 2.0 license
// https://github.com/docker/libnetwork/blob/master/LICENSE

import (
	"net"
)

// Proxy defines the behavior of a proxy. It forwards traffic back and forth
// between two endpoints : the frontend and the backend.
// It can be used to do software port-mapping between two addresses.
// e.g. forward all traffic between the frontend (host) 127.0.0.1:3000
// to the backend (container) at 172.17.42.108:4000.
type Proxy interface {
	// Run starts forwarding traffic back and forth between the front
	// and back-end addresses.
	Run()
	// Close stops forwarding traffic and close both ends of the Proxy.
	Close()
	// FrontendAddr returns the address on which the proxy is listening.
	FrontendAddr() net.Addr
	// BackendAddr returns the proxied address.
	BackendAddr() net.Addr
}

// NewProxy creates a Proxy according to the specified frontendAddr and backendAddr.
func NewProxy(frontendAddr, backendAddr net.Addr, envoySocketPath string) (Proxy, error) {
	switch frontendAddr.(type) {
	case *net.UDPAddr:
		return NewUDPProxy(frontendAddr.(*net.UDPAddr), backendAddr.(*net.UDPAddr))
	case *net.TCPAddr:
		return NewEnvoyProxy(frontendAddr.(*net.TCPAddr), backendAddr.(*net.TCPAddr), envoySocketPath)
	default:
		panic("Unsupported protocol")
	}
}
