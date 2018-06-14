package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const (
	ShimSocketPath = "/tmp/docker-envoy.sock"
)

// parseHostContainerAddrs parses the flags passed on reexec to create the TCP/UDP/SCTP
// net.Addrs to map the host and container ports
func parseCLI() (host net.Addr, container net.Addr, doReload bool) {
	var (
		proto         = flag.String("proto", "tcp", "proxy protocol")
		hostIP        = flag.String("host-ip", "", "host ip")
		hostPort      = flag.Int("host-port", -1, "host port")
		containerIP   = flag.String("container-ip", "", "container ip")
		containerPort = flag.Int("container-port", -1, "container port")
		reload        = flag.Bool("reload", false, "reload existing containers")
	)

	flag.Parse()

	switch *proto {
	case "tcp":
		host = &net.TCPAddr{IP: net.ParseIP(*hostIP), Port: *hostPort}
		container = &net.TCPAddr{IP: net.ParseIP(*containerIP), Port: *containerPort}
	case "udp":
		host = &net.UDPAddr{IP: net.ParseIP(*hostIP), Port: *hostPort}
		container = &net.UDPAddr{IP: net.ParseIP(*containerIP), Port: *containerPort}
	default:
		log.Fatalf("unsupported protocol %s", *proto)
	}

	return host, container, *reload
}

func handleStopSignals(p Proxy) {
	s := make(chan os.Signal, 10)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)

	for range s {
		p.Close()

		os.Exit(0)
	}
}

func main() {
	f := os.NewFile(3, "signal-parent")
	host, container, reload := parseCLI()

	var p Proxy
	var err error
	if reload {
		p, err = NewEnvoyProxy(host, container, ShimSocketPath)
		envoy := p.(*EnvoyProxy)
		envoy.Reload = true
	} else {
		p, err = NewProxy(host, container, ShimSocketPath)
		go handleStopSignals(p)
	}

	if err != nil {
		fmt.Fprintf(f, "1\n%s", err)
		f.Close()
		os.Exit(1)
	}

	// If we were run by Docker this will be open, if not, skip
	if f != nil {
		fmt.Fprint(f, "0\n")
		f.Close()
	}

	// Run will block until the proxy stops
	p.Run()
}
