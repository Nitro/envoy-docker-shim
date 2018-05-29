package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nitro/envoy-docker/envoyhttp"
	"github.com/Nitro/envoy-docker/envoyrpc"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	SocketAddr = "/tmp/docker-envoy.sock"
)

func handleStopSignals() {
	s := make(chan os.Signal, 10)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)

	<-s
	err := os.Remove(SocketAddr)
	if err != nil {
		log.Fatal("Unable to remove socket! (" + SocketAddr + ")")
	}
	log.Printf("Removed %s", SocketAddr)
	os.Exit(0)
}

func serveHttp(envoyApi *envoyhttp.EnvoyApi, addr string) {
	router := mux.NewRouter()

	router.PathPrefix("/v1").Handler(http.StripPrefix("/v1", envoyApi.HttpMux()))

	http.Handle("/", router)

	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("Can't start Envoy xDS API server: %s", err)
	}
}

func main() {
	lis, err := net.Listen("unix", SocketAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	go handleStopSignals()

	s := grpc.NewServer()
	registrar := envoyrpc.NewRegistrar()
	envoyrpc.RegisterRegistrarServer(s, registrar)

	api := envoyhttp.NewEnvoyApi(registrar)
	go serveHttp(api, ":7776")

	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
