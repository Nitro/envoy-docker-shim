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

Operations
----------

The command line tool doesn't require any special operations. However, the
other two components require a little attention.

### Server

You will need to start the server part of this application on the Docker host
itself. We'll be building a container for this, but for now you need to just
put the binary somewhere and get it to start.

### Resync

To prevent issues with getting out of sync with reality, the server only stores
state in memory. Upon restart of the service, when existing copies of the shim
are running for containers, you need to run the `resync` script, which will
lookin the process table and replay entries for existing containers. If you run
the server from systemd or another process manager, you should make sure that
the resync is run when the service is up.

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

Example Configuration
---------------------

We've included an exmaple `envoy.yaml` in the [examples](./examples) directory,
which will get you up and running with Envoy (tested on 1.6, 1.7). This should
work with the upstream Envoy container, or with [Nitro's Envoy
container](https://hub.docker.com/r/gonitro/envoyproxy/)

Contributing
------------

Contributions are more than welcome. Bug reports with specific reproduction
steps are great. If you have a code contribution you'd like to make, open a
pull request with suggested code.

Pull requests should:

 * Clearly state their intent in the title
 * Have a description that explains the need for the changes
 * Include tests!
 * Not break the public API

Ping us to let us know you're working on something interesting by opening a
GitHub Issue on the project.
