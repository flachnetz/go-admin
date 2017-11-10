package admin

import (
	"net/http"
	"strings"
)

type HTTPRouter interface {
	Handler(method, path string, handler http.Handler)
}

func SetupAdminHandlerHTTPRouter(router HTTPRouter, prefix, name string, routeConfigs ...RouteConfig) {
	prefix = "/" + strings.Trim(prefix, "/")

	admin := NewAdminHandler(prefix, name, routeConfigs...)

	methods := []string{"GET", "POST", "HEAD", "PUT", "DELETE", "OPTIONS", "PATCH"}
	for _, method := range methods {
		router.Handler(method, prefix, admin)
		router.Handler(method, prefix+"/*path", admin)
	}
}

func SetupAdminHandlerMux(mux *http.ServeMux, prefix, name string, routeConfigs ...RouteConfig) {
	admin := NewAdminHandler(prefix, name, routeConfigs...)
	mux.Handle(prefix, admin)
}
