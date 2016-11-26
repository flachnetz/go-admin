package admin

import (
	"net/http"
	"time"
	"runtime"
	"fmt"
	"io/ioutil"
	"os"
	"runtime/debug"
	"io"
	"github.com/kardianos/osext"
	"runtime/pprof"
	pprofH "net/http/pprof"
	"github.com/goji/httpauth"
	"strings"
	"log"
	"bytes"
	"html/template"
	"sort"
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

func NewAdminPageHandler(appName, prefix string, routes... RouteConfig) *adminContext {
	admin := &adminContext{appName: appName, prefix: prefix}
	admin.addRouteConfig(RouteConfig{children: routes})

	// add overview page
	admin.addRouteConfig(RouteConfig{Route: Route{
		Handler: admin.indexHandler(),
		Path: "/",
	}})

	return admin
}

func (admin*adminContext) addRouteConfig(config RouteConfig) {
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
					Name: route.Path,
					Path: pathOf(a.prefix, route.Path),
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
					http.Error(w, "Illegale method for this path, allowed: " + route.Method, http.StatusMethodNotAllowed)
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

func WithGeneric(path string, value interface{}) RouteConfig {
	return WithGetHandler(path, genericContentAsJSON(value))
}

func WithGetHandlerFunc(path string, handler http.HandlerFunc) RouteConfig {
	return WithGetHandler(path, handler)
}

func WithGetHandler(path string, handler http.Handler) RouteConfig {
	return WithHandler("GET", path, handler)
}

func WithHandlerFunc(method, path string, handler http.HandlerFunc) RouteConfig {
	return WithHandler(method, path, handler)
}

func WithHandler(method, path string, handler http.Handler) RouteConfig {
	return RouteConfig{Route: Route{Method: method, Path: path, Handler: handler}}
}

func WithMetrics(registry MetricsRegistry) RouteConfig {
	return Describe(
		"The current content of the MetricsRegistry",
		WithGeneric("/metrics", registry))
}

func WithForceGC() RouteConfig {
	return Describe(
		"Forces a run of the garbage collector.",
		WithHandlerFunc("POST", "/gc/run", func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()

			// do a gc now.
			runtime.GC()

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("gc took %s", time.Since(start))))
		}))
}

var appStartTime = time.Now()

func WithBuildInfo(buildInfo BuildInfo) RouteConfig {
	type appInfoWithTime struct {
		BuildInfo
		Hostname   string `json:",omitempty"`
		StartTime  time.Time
		ServerTime time.Time
		Uptime     string
	}

	return Describe(
		"Information about the current build",
		WithGeneric("/info", func() appInfoWithTime {
			hostname, _ := os.Hostname()
			return appInfoWithTime{
				BuildInfo:  buildInfo,
				Hostname: hostname,
				StartTime:  appStartTime,
				ServerTime: time.Now(),
				Uptime:     time.Since(appStartTime).String(),
			}
		}))
}

func WithHeapDump() RouteConfig {
	return Describe(
		"Creates a snapshot of the processes heap.",
		WithHandlerFunc("GET", "pprof/heapdump", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")

			// we can not write the heap-dump directly to the network socket,
			// so write it down to disk first.
			file, err := ioutil.TempFile("/tmp/", "heapdump")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// cleanup after ourselves
			defer os.Remove(file.Name())
			defer file.Close()

			// write the dump to a file
			debug.WriteHeapDump(file.Fd())

			// looks good, serve the file.
			filename := fmt.Sprintf("heapdump-%s.heap", time.Now().Format("20060102-150405"))
			w.Header().Set("Content-Disposition", "attachment; filename=" + filename)

			file.Seek(0, os.SEEK_SET)
			io.Copy(w, file)
		}))
}

func WithPProfHandlers() RouteConfig {
	rc := RouteConfig{children: []RouteConfig{
		Describe(
			"The command line of the running process. Arguments are seperated by null bytes.",
			WithHandler("GET", "pprof/cmdline", http.HandlerFunc(pprofH.Cmdline))),

		Describe(
			"Profiles the application. Use with 'go pprof http://host/pprof/profile'",
			WithHandler("GET", "pprof/profile", http.HandlerFunc(pprofH.Profile))),

		Describe(
			"Performes a trace of cpu, io and more. Accepts an url parameter 'seconds'",
			WithHandler("GET", "pprof/trace", http.HandlerFunc(pprofH.Trace))),

		// symbol can handle post data, register it for GET and POST.
		Describe(
			"Resolves addresses to symbols. Used by ppprof.",
			WithHandler("", "pprof/symbol", http.HandlerFunc(pprofH.Symbol))),

		Describe(
			"Provides a memory profile of the application as done by 'WriteHeapProfile'",
			WithHandlerFunc("GET", "pprof/memprofile", func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/octet-stream")
				pprof.WriteHeapProfile(w)
			})),
	}}

	// also expose the currently used binary - this simplifies profiling.
	if exe, err := osext.Executable(); err == nil {
		rc.children = append(rc.children, Describe(
			"Downloads the process binary that is currently running",
			WithHandlerFunc("GET", "pprof/exe", func(w http.ResponseWriter, req *http.Request) {
				http.ServeFile(w, req, exe)
			})))
	}

	return rc
}

func WithEnvironmentVariables() RouteConfig {
	return Describe(
		"A map containing all environment variables.",
		WithGeneric("env", os.Environ))
}

func RequireAuth(user, pass string, configs... RouteConfig) RouteConfig {
	auth := httpauth.SimpleBasicAuth(user, pass)

	var secured []RouteConfig
	for _, config := range configs {
		config.Handler = auth(config.Handler)
		secured = append(secured, config)
	}

	return RouteConfig{children: secured}
}

func WithPingPong() RouteConfig {
	return Describe(
		"Always returns the static json '{\"pong\": true}'",
		WithGeneric("/ping", map[string]bool{"pong": true}))
}

func WithGCStats() RouteConfig {
	return Describe(
		"Displays the current gc- and memory-statistics from the golang runtime.",
		WithGeneric("/gc/stats", func() interface{} {
			var stats struct {
				GarbageCollectorStats debug.GCStats
				MemStats              runtime.MemStats
			}

			debug.ReadGCStats(&stats.GarbageCollectorStats)
			runtime.ReadMemStats(&stats.MemStats)

			return stats
		}))
}

func WithDefaults() RouteConfig {
	return RouteConfig{children: []RouteConfig{
		WithPingPong(),
		WithEnvironmentVariables(),
		WithGCStats(),
	}}
}
