package envoyhttp

//go:generate ffjson $GOFILE

// Envoy API definitions --------------------------------------------------

// See https://www.envoyproxy.io/docs/envoy/latest/api-v1/cluster_manager/sds.html
type EnvoyService struct {
	IPAddress       string            `json:"ip_address"`
	LastCheckIn     string            `json:"last_check_in"`
	Port            int64             `json:"port"`
	Revision        string            `json:"revision"`
	Service         string            `json:"service"`
	ServiceRepoName string            `json:"service_repo_name"`
	Tags            map[string]string `json:"tags"`
}

// See https://www.envoyproxy.io/docs/envoy/latest/api-v1/cluster_manager/cluster.html
type EnvoyCluster struct {
	Name             string `json:"name"`
	Type             string `json:"type"`
	ConnectTimeoutMs int64  `json:"connect_timeout_ms"`
	LBType           string `json:"lb_type"`
	ServiceName      string `json:"service_name"`
	// Many optional fields omitted
}

// https://www.envoyproxy.io/docs/envoy/latest/api-v1/listeners/listeners.html
type EnvoyListener struct {
	Name    string         `json:"name"`
	Address string         `json:"address"`
	Filters []*EnvoyFilter `json:"filters"`
	// Many optional fields omitted
}

// A basic Envoy Route Filter
type EnvoyFilter struct {
	Name   string                 `json:"name"`
	Config *EnvoyFilterConfig `json:"config"`
}

type EnvoyFilterConfig struct {
	CodecType   string              `json:"codec_type,omitempty"`
	StatPrefix  string              `json:"stat_prefix,omitempty"`
	RouteConfig *EnvoyRouteConfig   `json:"route_config,omitempty"`
	Filters     []*EnvoyFilter      `json:"filters,omitempty"`
	Tracing     *EnvoyTracingConfig `json:"tracing,omitempty"`
}

type EnvoyHTTPVirtualHost struct {
	Name    string        `json:"name"`
	Domains []string      `json:"domains"`
	Routes  []*EnvoyRoute `json:"routes"`
}

type EnvoyRouteConfig struct {
	VirtualHosts []*EnvoyHTTPVirtualHost `json:"virtual_hosts,omitempty"` // Used for HTTP
	Routes       []*EnvoyTCPRoute        `json:"routes,omitempty"` // Use for TCP
}

type EnvoyRoute struct {
	TimeoutMs   int                  `json:"timeout_ms"`
	Prefix      string               `json:"prefix"`
	HostRewrite string               `json:"host_rewrite"`
	Cluster     string               `json:"cluster"`
	Decorator   *EnvoyRouteDecorator `json:"decorator,omitempty"`
}

type EnvoyRouteDecorator struct {
	Operation string `json:"operation,omitempty"`
}

type EnvoyTracingConfig struct {
	OperationName string `json:"operation_name"`
}

type EnvoyTCPRoute struct {
	Cluster           string   `json:"cluster"`
	DestinationIPList []string `json:"destination_ip_list,omitempty"`
	DestinationPorts  string   `json:"destination_ports,omitempty"`
	SourceIPList      []string `json:"source_ip_list,omitempty"`
	SourcePorts       []string `json:"source_ports,omitempty"`
}

type SDSResult struct {
	Env     string          `json:"env"`
	Hosts   []*EnvoyService `json:"hosts"`
	Service string          `json:"service"`
}

type CDSResult struct {
	Clusters []*EnvoyCluster `json:"clusters"`
}

type LDSResult struct {
	Listeners []*EnvoyListener `json:"listeners"`
}

// ------------------------------------------------------------------------
