Envoy Docker Shim
=================

This is a pre-production project to use Envoy in place of Docker's own
`docker-proxy`. The point of doing this is to enable Envoy's metric gathering
and distributed tracing capabilities for any service running on Docker.
Essentially if you run this, you get half of a basic service mesh almost for
free.  There are four parts to the system:

1. A registrar service that runs on the Docker host and serves the discovery
   APIs to Envoy.
2. A command line application that takes the same arguments as Docker's
   native `docker-proxy` command but registers the new endpoint with
   the registrar. It talks to the registrar over a Unix socket.
3. An instance of Envoy, normally itself running inside a Docker container
   in host networking mode.
4. A shell script which can restore the state in the registrar in the event
   that it needs to be restarted while containers are running.

Together these form a system which allows Envoy to handle both HTTP and TCP
proxying duties and the command line tool continues to handle UDP traffic using
the code from `docker-proxy`. Currently SCTP is not supported.

Installation
------------

You must run `dockerd` with the following settings:

* `--userland-proxy-path=/path/to/envoy-docker-shim`: Replaces `docker-proxy`
  with the shim from this project, in order to tell Envoy what to listen on
  and where to forward to.
* `--iptables=false`: Disables IPTables forwarding of Docker traffic,
  thereby forcing everything over Envoy.

Once you've enabled the `--iptables=false` setting, you will no longer be
allowing traffic to flow into the bridged network directly via the Kernel.  All
traffic will be proxied at Layer 4 or 7, depending on which mode you are
proxying.

Container Settings
------------------

This project currently assumes that you will have set at least two, and
possibly three Docker labels on each of your containers. These are _not_
required, but do improve the experience of using this project, particularly
around the area of distributed tracing. The Docker labels are used to create
the tags that get applied to services when reported to Zipkin or Jaeger. The
three labels are inherited from [Sidecar](https://github.com/Nitro/sidecar) and
are:

* `EnvironmentName`: Intended to be something that is meaningful to you. It could
  be a customer name, or something like `production` or `staging`.
* `ServiceName`: How you want to name the service in traces. This normally does not
  include the name of the environment. Thus if your service is named `nginx-prod`
  you would probably just name it `nginx` in this label.
* `ProxyMode`: This shim assumes that you will be running in `http` proxy mode
  and that is the default value for this label if you don't provide it. If you
  instead want Envoy to proxy TCP traffic, you need to provide the value `tcp`.
