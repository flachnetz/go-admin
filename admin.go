package admin

import (
	"bytes"
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
	wildcard    bool
}

type RouteConfig struct {
	Route
	children []RouteConfig
}

func (rc RouteConfig) Describe(description string) RouteConfig {
	rc.Description = description
	return rc
}

func (rc RouteConfig) Wildcard(wildcard bool) RouteConfig {
	rc.wildcard = wildcard
	return rc
}

func Describe(desc string, rc RouteConfig) RouteConfig {
	return rc.Describe(desc)
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
					Path:        strings.TrimLeft(pathOf(a.prefix, route.Path), "/"),
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

		if path != req.URL.Path {
			http.Redirect(w, req, path, http.StatusTemporaryRedirect)
			return
		}

		// remove prefix from path and apply to request
		path = pathOf(strings.TrimPrefix(path, admin.prefix))
		req.URL.Path = path

		for _, route := range admin.routes {
			if routePathMatches(route, path) {
				if isCompatibleMethod(route.Method, req.Method) {
					// forward request to the handler
					route.Handler.ServeHTTP(w, req)

				} else {
					http.Error(w, "Illegale method for this path, allowed: "+route.Method, http.StatusMethodNotAllowed)
				}

				return
			}
		}

		http.NotFound(w, req)
	}
}

func routePathMatches(route Route, path string) bool {
	routePath := pathOf("/", route.Path)

	if route.wildcard {
		return strings.HasPrefix(path, routePath)
	} else {
		return routePath == path
	}
}

func isCompatibleMethod(expected, actual string) bool {
	return expected == "" || expected == actual || expected == "GET" && actual == "HEAD"
}
