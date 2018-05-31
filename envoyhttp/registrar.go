package envoyhttp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/Nitro/envoy-docker-shim/shimrpc"
)

type Entry struct {
	FrontendAddr    *net.TCPAddr
	BackendAddr     *net.TCPAddr
	ServiceName     string
	EnvironmentName string
}

type Registrar struct {
	sync.RWMutex
	entries map[string]*Entry
}

func NewRegistrar() *Registrar {
	return &Registrar{
		entries: make(map[string]*Entry),
	}
}

func (r *Registrar) PrintRequests() {
	log.Println("Requests:")
	for n, entry := range r.entries {
		log.Printf("%s: %#v\n", n, *entry)
	}
}

// GetRequest looks up on request from the map and returns a pointer
// if present.
func (r *Registrar) GetEntry(svcName string) *Entry {
	r.RLock()
	defer r.RUnlock()

	return r.entries[svcName]
}

// EachEntry iterates the entries, calling the passed function on each.
func (r *Registrar) EachEntry(fn func(svcName string, entry *Entry) error) error {
	r.RLock()
	defer r.RUnlock()

	for svcName, entry := range r.entries {
		err := fn(svcName, entry)
		if err != nil {
			return err
		}
	}

	return nil
}

func RequestToEntry(req *shimrpc.RegistrarRequest) *Entry {
	return &Entry{
		FrontendAddr: &net.TCPAddr{
			IP:   net.ParseIP(req.FrontendAddr),
			Port: int(req.FrontendPort),
		},
		BackendAddr: &net.TCPAddr{
			IP:   net.ParseIP(req.BackendAddr),
			Port: int(req.BackendPort),
		},
		ServiceName:     req.ServiceName,
		EnvironmentName: req.EnvironmentName,
	}
}

// Format an Envoy service name from an endpoint
func SvcName(entry *Entry) string {
	var svcName string

	if len(entry.ServiceName) > 0 {
		svcName = entry.ServiceName
	}

	if len(entry.EnvironmentName) > 0 {
		svcName += "-" + entry.EnvironmentName
	}

	if len(svcName) < 1 {
		svcName = "unknown-"
	}

	return fmt.Sprintf("%s%d", svcName, entry.FrontendAddr.Port)
}

// Register is a GRPC callback function that handles our remote calls.
func (r *Registrar) Register(ctx context.Context, req *shimrpc.RegistrarRequest) (*shimrpc.RegistrarReply, error) {
	// Register a new endpoint
	if req.Action == shimrpc.RegistrarRequest_REGISTER {
		entry := RequestToEntry(req)
		name := SvcName(entry)

		log.Printf("Registering %s\n", name)
		r.Lock()
		r.entries[name] = entry
		r.PrintRequests()
		r.Unlock()
		return &shimrpc.RegistrarReply{1}, nil
	}

	// Deregister an endpoint
	if req.Action == shimrpc.RegistrarRequest_DEREGISTER {
		entry := RequestToEntry(req)
		name := SvcName(entry)

		log.Printf("Deregistering %s\n", name)
		r.Lock()
		delete(r.entries, name)
		r.PrintRequests()
		r.Unlock()
		return &shimrpc.RegistrarReply{1}, nil
	}

	// Who knows what we were asked to due, but we're not dpoing it
	return &shimrpc.RegistrarReply{0}, errors.New("Unknown request action. No idea what to do with it.")
}
