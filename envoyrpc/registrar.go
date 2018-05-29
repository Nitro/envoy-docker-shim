package envoyrpc

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"
)

type Entry struct {
	FrontendAddr *net.TCPAddr
	BackendAddr  *net.TCPAddr
}

type Registrar struct {
	sync.RWMutex
	entries map[int32]*Entry
}

func NewRegistrar() *Registrar {
	return &Registrar{
		entries: make(map[int32]*Entry),
	}
}

func (r *Registrar) PrintRequests() {
	log.Println("Requests:")
	for p, req := range r.entries {
		log.Printf("%d: %#v\n", p, *req)
	}
}

// GetRequest looks up on request from the map and returns a pointer
// if present.
func (r *Registrar) GetEntry(port int32) *Entry {
	r.RLock()
	defer r.RUnlock()

	return r.entries[port]
}

// EachEntry iterates the entries, calling the passed function on each.
func (r *Registrar) EachEntry(fn func(port int32, entry *Entry) error) error {
	r.RLock()
	defer r.RUnlock()

	for port, entry := range r.entries {
		err := fn(port, entry)
		if err != nil {
			return err
		}
	}

	return nil
}

func RequestToEntry(req *RegistrarRequest) *Entry {
	return &Entry{
		FrontendAddr: &net.TCPAddr{
			IP:   net.ParseIP(req.FrontendAddr),
			Port: int(req.FrontendPort),
		},
		BackendAddr: &net.TCPAddr{
			IP:   net.ParseIP(req.BackendAddr),
			Port: int(req.BackendPort),
		},
	}
}

// Register is a GRPC callback function that handles our remote calls.
func (r *Registrar) Register(ctx context.Context, req *RegistrarRequest) (*RegistrarReply, error) {
	// Register a new endpoint
	if req.Action == RegistrarRequest_REGISTER {
		log.Printf("Registering %d\n", req.FrontendPort)
		r.Lock()
		r.entries[req.FrontendPort] = RequestToEntry(req)
		r.PrintRequests()
		r.Unlock()
		return &RegistrarReply{1}, nil
	}

	// Deregister an endpoint
	if req.Action == RegistrarRequest_DEREGISTER {
		log.Printf("Deregistering %d\n", req.FrontendPort)
		r.Lock()
		delete(r.entries, req.FrontendPort)
		r.PrintRequests()
		r.Unlock()
		return &RegistrarReply{1}, nil
	}

	// Who knows what we were asked to due, but we're not dpoing it
	return &RegistrarReply{0}, errors.New("Unknown request action. No idea what to do with it.")
}
