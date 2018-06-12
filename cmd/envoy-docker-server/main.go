package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Nitro/envoy-docker-shim/internal/envoyhttp"
	"github.com/Nitro/envoy-docker-shim/internal/shimrpc"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gopkg.in/relistan/rubberneck.v1"
)

type Config struct {
	GrpcAddr string `envconfig:"LISTEN_ADDR" default:"unix:///tmp/docker-envoy.sock"`
	ApiAddr    string `envconfig:"API_ADDR" default:":7776"`
}

func handleStopSignals(addr string) {
	s := make(chan os.Signal, 10)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)

	<-s

	if strings.Contains(addr, "unix:") {
		err := os.Remove(strings.Replace(addr, "unix://", "", 1))
		if err != nil {
			log.Fatal("Unable to remove socket! (" + addr + ")")
		}
		log.Printf("Removed %s", addr)
	}
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

func serveGRPC(registrar *envoyhttp.Registrar, addr string) {
	addr = strings.Replace(addr, "unix://", "", 1)

	lis, err := net.Listen("unix", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	shimrpc.RegisterRegistrarServer(s, registrar)

	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	log.Info("docker-envoy-shim server starting up...")

	var config Config
	err := envconfig.Process("shim", &config)
	if err != nil {
		log.Fatal(err)
	}

	rubberneck.Print(&config)

	go handleStopSignals(config.GrpcAddr)

	registrar := envoyhttp.NewRegistrar()
	api := envoyhttp.NewEnvoyApi(registrar)
	go serveHttp(api, config.ApiAddr)

	serveGRPC(registrar, config.GrpcAddr)
}
