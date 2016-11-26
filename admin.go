package admin

import (
	"bytes"
	"github.com/gorilla/mux"
	"html/template"
	"net/http"
	"sort"
	"strings"
)

type Route struct {
	Handler     http.Handler
	Method      string
	Path        string
	Description string
}

type RouteConfig struct {
	Route
	children []RouteConfig
}

func Describe(desc string, rc RouteConfig) RouteConfig {
	rc.Description = desc
	return rc
}

type adminContext struct {
	appName string
	prefix  string
	routes  []Route
}

func NewAdminHandler(prefix, appName string, routes ...RouteConfig) http.Handler {
	admin := &adminContext{appName: appName, prefix: prefix}
	admin.addRouteConfig(RouteConfig{children: routes})

	// add overview page
	admin.addRouteConfig(RouteConfig{Route: Route{
		Handler: admin.indexHandler(),
		Path:    "/",
	}})

	return admin.AsHandler()
}

func AddAdminHandler(router *mux.Router, prefix, appName string, routes ...RouteConfig) {
	adm := NewAdminHandler(prefix, appName, routes...)
	router.PathPrefix(prefix).Handler(adm)
}

func AddIndexAdminHandler(router *mux.Router, prefix, appName string, routes ...RouteConfig) {
	router.Path("/").Handler(http.RedirectHandler(prefix, http.StatusTemporaryRedirect))
	AddAdminHandler(router, prefix, appName, routes...)
}

func (admin *adminContext) addRouteConfig(config RouteConfig) {
	for _, route := range config.children {
		admin.addRouteConfig(route)
	}

	if config.Path != "" {
		route := config.Route
		route.Path = pathOf(config.Path)
		route.Method = strings.ToUpper(config.Method)
		admin.routes = append(admin.routes, route)
	}
}

// Creates a handler that can handle multiple pages that are given by the pages map.
// The map must contain paths (like /metrics) to specific handlers for those paths.
// A index page will be created with links to all sub-paths.
func (a *adminContext) indexHandler() http.HandlerFunc {
	// compile template
	tmpl, err := template.New("adminIndex").Parse(indexTemplate)

	if err != nil {
		// should never happen!
		panic(err)
	}

	// add index handler
	return func(w http.ResponseWriter, r *http.Request) {
		var links linkSlice
		for _, route := range a.routes {
			if route.Path != "/" {
				links = append(links, link{
					Name:        route.Path,
					Path:        pathOf(a.prefix, route.Path),
					Description: route.Description,
				})
			}
		}

		// sort them by alphabet.
		sort.Sort(links)

		templateContext := indexContext{
			Links:   links,
			AppName: a.appName,
		}

		// render template
		buffer := &bytes.Buffer{}
		if err := tmpl.Execute(buffer, templateContext); err == nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write(buffer.Bytes())

		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (admin *adminContext) AsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		path := pathOf(req.URL.Path)

		for _, route := range admin.routes {
			if pathOf(admin.prefix, route.Path) == path {
				if route.Method != "" && route.Method != req.Method {
					http.Error(w, "Illegale method for this path, allowed: "+route.Method, http.StatusMethodNotAllowed)
					return
				}

				// forward request to the handler
				route.Handler.ServeHTTP(w, req)
				return
			}
		}

		http.NotFound(w, req)
	}
}
