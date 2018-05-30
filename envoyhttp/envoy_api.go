package envoyhttp

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/Nitro/envoy-docker-shim/shimrpc"
	"github.com/gorilla/mux"
	"github.com/pquerna/ffjson/ffjson"
	log "github.com/sirupsen/logrus"
)

type EnvoyApi struct {
	registrar *shimrpc.Registrar
}

func NewEnvoyApi(registrar *shimrpc.Registrar) *EnvoyApi {
	return &EnvoyApi{
		registrar: registrar,
	}
}

// optionsHandler sends CORS headers
func (s *EnvoyApi) optionsHandler(response http.ResponseWriter, req *http.Request) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	response.Header().Set("Access-Control-Allow-Methods", "GET")
	return
}

// registrationHandler takes the name of a single service and returns results for just
// that service. It implements the Envoy SDS API V1.
func (s *EnvoyApi) registrationHandler(response http.ResponseWriter, req *http.Request, params map[string]string) {
	defer req.Body.Close()

	response.Header().Set("Content-Type", "application/json")

	name, ok := params["service"]
	if !ok {
		log.Debug("No service name provided to Envoy registrationHandler")
		sendJsonError(response, 404, "Not Found - No service name provided")
		return
	}

	entry := s.registrar.GetEntry(name)

	if entry == nil {
		log.Debugf("Envoy Service '%s' has no instances!", name)
		sendJsonError(response, 404, fmt.Sprintf("no instances of '%s' found", name))
		return
	}

	instances := []*EnvoyService{
		s.EnvoyServiceFromEntry(entry),
	}

	// Did we have any entries for this service in the catalog?
	if len(instances) == 0 {
		log.Debugf("Envoy Service '%s' has no instances!", name)
		sendJsonError(response, 404, fmt.Sprintf("no instances of %s found", name))
		return
	}

	result := SDSResult{
		Hosts:   instances,
		Service: name,
	}

	jsonBytes, err := result.MarshalJSON()
	defer ffjson.Pool(jsonBytes)
	if err != nil {
		log.Errorf("Error marshaling state in registrationHandler: %s", err.Error())
		sendJsonError(response, 500, "Internal server error")
		return
	}

	response.Write(jsonBytes)
}

// clustersHandler returns cluster information for all Sidecar services. It
// implements the Envoy CDS API V1.
func (s *EnvoyApi) clustersHandler(response http.ResponseWriter, req *http.Request, params map[string]string) {
	defer req.Body.Close()

	response.Header().Set("Content-Type", "application/json")

	clusters := s.EnvoyClustersFromRegistrar()

	log.Debugf("Reporting Envoy cluster information for cluster '%s' and node '%s'",
		params["service_cluster"], params["service_node"])

	result := CDSResult{clusters}

	jsonBytes, err := result.MarshalJSON()
	defer ffjson.Pool(jsonBytes)
	if err != nil {
		log.Errorf("Error marshaling state in servicesHandler: %s", err.Error())
		sendJsonError(response, 500, "Internal server error")
		return
	}

	response.Write(jsonBytes)
}

// listenersHandler returns a list of listeners for all ServicePorts. It
// implements the Envoy LDS API V1.
func (s *EnvoyApi) listenersHandler(response http.ResponseWriter, req *http.Request, params map[string]string) {
	defer req.Body.Close()

	response.Header().Set("Content-Type", "application/json")

	log.Debugf("Reporting Envoy cluster information for cluster '%s' and node '%s'",
		params["service_cluster"], params["service_node"])

	listeners := s.EnvoyListenersFromRegistrar()

	result := LDSResult{listeners}
	jsonBytes, err := result.MarshalJSON()
	defer ffjson.Pool(jsonBytes)
	if err != nil {
		log.Errorf("Error marshaling state in servicesHandler: %s", err.Error())
		sendJsonError(response, 500, "Internal server error")
		return
	}

	response.Write(jsonBytes)
}

// lookupHost does a vv slow lookup of the DNS host for a service. Totally
// not optimized for high throughput. You should only do this in development
// scenarios.
func lookupHost(hostname string) (string, error) {
	addrs, err := net.LookupHost(hostname)

	if err != nil {
		return "", err
	}
	return addrs[0], nil
}

// EnvoyServiceFromRequest converts a Registrar request to an Envoy
// API service for reporting to the proxy.
func (s *EnvoyApi) EnvoyServiceFromEntry(entry *shimrpc.Entry) *EnvoyService {
	if entry == nil {
		return nil
	}

	return &EnvoyService{
		IPAddress:       entry.BackendAddr.IP.String(),
		LastCheckIn:     time.Now().UTC().String(),
		Port:            int64(entry.BackendAddr.Port),
		Revision:        "1",
		Service:         shimrpc.SvcName(entry),
		ServiceRepoName: "docker service",
		Tags:            map[string]string{},
	}
}

// EnvoyClustersFromRegistrar genenerates a set of Envoy API cluster
// definitions from Registrar state.
func (s *EnvoyApi) EnvoyClustersFromRegistrar() []*EnvoyCluster {
	var clusters []*EnvoyCluster

	s.registrar.EachEntry(func(name string, entry *shimrpc.Entry) error {
		clusters = append(clusters, &EnvoyCluster{
			Name:             shimrpc.SvcName(entry),
			Type:             "sds", // use SDS endpoint for the hosts
			ConnectTimeoutMs: 500,
			LBType:           "round_robin",
			ServiceName:      shimrpc.SvcName(entry),
		})

		return nil
	})

	if clusters == nil {
		clusters = []*EnvoyCluster{}
	}

	return clusters
}

// EnvoyListenerFromEntry takes a Registrar request service and formats it into
// the API format for an Envoy proxy listener (LDS API v1)
func (s *EnvoyApi) EnvoyListenerFromEntry(entry *shimrpc.Entry) *EnvoyListener {
	apiName := shimrpc.SvcName(entry)

	// Holy indentation, Bat Man!
	return &EnvoyListener{
		Name:    apiName,
		Address: fmt.Sprintf("tcp://%s:%d", entry.FrontendAddr.IP, entry.FrontendAddr.Port),
		Filters: []*EnvoyFilter{
			{
				Name: "envoy.http_connection_manager",
				Config: &EnvoyHttpFilterConfig{
					CodecType:  "auto",
					StatPrefix: "ingress_http",
					Filters: []*EnvoyFilter{
						{
							Name:   "router",
							Config: &EnvoyHttpFilterConfig{},
						},
					},
					RouteConfig: &EnvoyRouteConfig{
						VirtualHosts: []*EnvoyVirtualHost{
							{
								Name:    shimrpc.SvcName(entry),
								Domains: []string{"*"},
								Routes: []*EnvoyRoute{
									{
										TimeoutMs: 0, // No timeout!
										Prefix:    "/",
										Cluster:   apiName,
									},
								},
							},
						},
					},
					Tracing: &EnvoyTracingConfig{
						OperationName: "egress",
					},
				},
			},
		},
	}
}

// EnvoyListenersFromRegistrar creates a set of Enovy API listener
// definitions from all the ports in the Registrar.
func (s *EnvoyApi) EnvoyListenersFromRegistrar() []*EnvoyListener {
	var listeners []*EnvoyListener

	s.registrar.EachEntry(func(name string, entry *shimrpc.Entry) error {
		listeners = append(listeners, s.EnvoyListenerFromEntry(entry))
		return nil
	})

	if listeners == nil {
		listeners = []*EnvoyListener{}
	}

	return listeners
}

// Send back a JSON encoded error and message
func sendJsonError(response http.ResponseWriter, status int, message string) {
	output := map[string]string{
		"status":  "error",
		"message": message,
	}

	jsonBytes, err := json.Marshal(output)

	if err != nil {
		log.Errorf("Error encoding json error response: %s", err.Error())
		response.WriteHeader(500)
		response.Write([]byte("Interval server error"))
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	response.Write(jsonBytes)
}

func wrap(fn func(http.ResponseWriter, *http.Request, map[string]string)) http.HandlerFunc {
	return func(response http.ResponseWriter, req *http.Request) {
		fn(response, req, mux.Vars(req))
	}
}

// HttpMux returns a configured Gorilla mux to handle all the endpoints
// for the Envoy API.
func (s *EnvoyApi) HttpMux() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/registration/{service}", wrap(s.registrationHandler)).Methods("GET")
	router.HandleFunc("/clusters/{service_cluster}/{service_node}", wrap(s.clustersHandler)).Methods("GET")
	router.HandleFunc("/clusters", wrap(s.clustersHandler)).Methods("GET")
	router.HandleFunc("/listeners/{service_cluster}/{service_node}", wrap(s.listenersHandler)).Methods("GET")
	router.HandleFunc("/listeners", wrap(s.listenersHandler)).Methods("GET")
	router.HandleFunc("/{path}", s.optionsHandler).Methods("OPTIONS")

	return router
}
