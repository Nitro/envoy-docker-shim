package envoyhttp

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nitro/envoy-docker-shim/shimrpc"
	. "github.com/smartystreets/goconvey/convey"
)

var (
	hostname = "chaucer"

	baseTime = time.Now().UTC()

	req1 = &shimrpc.RegistrarRequest{
		FrontendAddr:    "192.168.168.99",
		FrontendPort:    12345,
		BackendAddr:     "172.16.10.1",
		BackendPort:     80,
		EnvironmentName: "dev",
		ServiceName:     "bede",
		ProxyMode:       "tcp",
		Action:          shimrpc.RegistrarRequest_REGISTER,
	}

	req2 = &shimrpc.RegistrarRequest{
		FrontendAddr:    "192.168.168.98",
		FrontendPort:    23451,
		BackendAddr:     "172.16.10.2",
		BackendPort:     8080,
		EnvironmentName: "dev",
		ServiceName:     "chretien",
		ProxyMode:       "http",
		Action:          shimrpc.RegistrarRequest_REGISTER,
	}

	req3 = &shimrpc.RegistrarRequest{
		FrontendAddr:    "192.168.168.97",
		FrontendPort:    23555,
		BackendAddr:     "172.16.10.3",
		BackendPort:     9000,
		EnvironmentName: "dev",
		ServiceName:     "hakluyt",
		ProxyMode:       "http",
		Action:          shimrpc.RegistrarRequest_REGISTER,
	}
)

func Test_clustersHandler(t *testing.T) {
	Convey("clustersHandler()", t, func() {
		registrar := NewRegistrar()
		registrar.Register(context.Background(), req1)
		registrar.Register(context.Background(), req2)
		registrar.Register(context.Background(), req3)

		api := &EnvoyApi{registrar: registrar}

		req := httptest.NewRequest("GET", "/clusters", nil)
		recorder := httptest.NewRecorder()

		Convey("returns information for registered services", func() {
			api.clustersHandler(recorder, req, nil)
			status, _, body := getResult(recorder)

			So(status, ShouldEqual, 200)
			So(body, ShouldContainSubstring, "hakluyt")
		})

		Convey("does not include deregistered services", func() {
			req1.Action = shimrpc.RegistrarRequest_DEREGISTER
			registrar.Register(context.Background(), req1)

			api.clustersHandler(recorder, req, nil)
			status, _, body := getResult(recorder)

			So(status, ShouldEqual, 200)
			So(body, ShouldNotContainSubstring, "bede")
			req1.Action = shimrpc.RegistrarRequest_REGISTER
		})
	})
}

func Test_registrationHandler(t *testing.T) {
	Convey("registrationHandler()", t, func() {
		registrar := NewRegistrar()
		registrar.Register(context.Background(), req1)
		registrar.Register(context.Background(), req2)
		registrar.Register(context.Background(), req3)

		api := &EnvoyApi{registrar: registrar}

		recorder := httptest.NewRecorder()

		Convey("returns an error unless a service is provided", func() {
			req := httptest.NewRequest("GET", "/registration/", nil)
			api.registrationHandler(recorder, req, nil)
			status, _, _ := getResult(recorder)

			So(status, ShouldEqual, 404)
		})

		Convey("returns an error unless registered name is appended to the URL", func() {
			req := httptest.NewRequest("GET", "/registration/", nil)
			params := map[string]string{
				"service": "bocaccio",
			}
			api.registrationHandler(recorder, req, params)
			status, _, _ := getResult(recorder)

			So(status, ShouldEqual, 404)
		})

		Convey("returns information for registered endpoints", func() {
			req := httptest.NewRequest("GET", "/registration/bede-dev-12345", nil)
			params := map[string]string{
				"service": "bede-dev-12345",
			}
			api.registrationHandler(recorder, req, params)
			status, _, body := getResult(recorder)

			So(status, ShouldEqual, 200)
			So(body, ShouldContainSubstring, "bede")
		})

		Convey("does not include deregistered endpoints", func() {
			req1.Action = shimrpc.RegistrarRequest_DEREGISTER
			registrar.Register(context.Background(), req1)
			req1.Action = shimrpc.RegistrarRequest_REGISTER

			req := httptest.NewRequest("GET", "/registration/bede-dev-12345", nil)
			params := map[string]string{
				"service": "bede-dev-12345",
			}
			api.registrationHandler(recorder, req, params)
			status, _, body := getResult(recorder)

			So(status, ShouldEqual, 404)
			So(body, ShouldContainSubstring, "no instances of 'bede")
		})
	})
}

func Test_listenersHandler(t *testing.T) {
	Convey("listenersHandler()", t, func() {
		registrar := NewRegistrar()
		registrar.Register(context.Background(), req1)
		registrar.Register(context.Background(), req2)
		registrar.Register(context.Background(), req3)

		api := &EnvoyApi{registrar: registrar}

		recorder := httptest.NewRecorder()

		Convey("returns listeners for registered endpoints", func() {
			req := httptest.NewRequest("GET", "/listeners/", nil)
			api.listenersHandler(recorder, req, nil)
			status, _, body := getResult(recorder)

			So(status, ShouldEqual, 200)
			So(body, ShouldContainSubstring, "bede")
			So(body, ShouldContainSubstring, "hakluyt")
			So(body, ShouldContainSubstring, "chretien")
		})

		Convey("respects the ProxyMode setting when returning results", func() {

			Convey("for TCP mode", func() {
				req2.Action = shimrpc.RegistrarRequest_DEREGISTER
				registrar.Register(context.Background(), req2)
				req2.Action = shimrpc.RegistrarRequest_REGISTER

				req3.Action = shimrpc.RegistrarRequest_DEREGISTER
				registrar.Register(context.Background(), req3)
				req3.Action = shimrpc.RegistrarRequest_REGISTER

				req := httptest.NewRequest("GET", "/listeners/", nil)
				api.listenersHandler(recorder, req, nil)
				status, _, body := getResult(recorder)

				So(status, ShouldEqual, 200)
				So(body, ShouldContainSubstring, "bede")
				So(body, ShouldNotContainSubstring, "hakluyt")
				So(body, ShouldNotContainSubstring, "chretien")
			})

			Convey("for HTTP mode", func() {
				req1.Action = shimrpc.RegistrarRequest_DEREGISTER
				registrar.Register(context.Background(), req1)
				req1.Action = shimrpc.RegistrarRequest_REGISTER

				req := httptest.NewRequest("GET", "/listeners/", nil)
				api.listenersHandler(recorder, req, nil)
				status, _, body := getResult(recorder)

				So(status, ShouldEqual, 200)
				So(body, ShouldNotContainSubstring, "bede")
				So(body, ShouldContainSubstring, "hakluyt")
				So(body, ShouldContainSubstring, "chretien")
			})
		})
	})
}

// getResult fetchs the status code, headers, and body from a recorder
func getResult(recorder *httptest.ResponseRecorder) (code int, headers *http.Header, body string) {
	resp := recorder.Result()
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	body = string(bodyBytes)

	return resp.StatusCode, &resp.Header, body
}
