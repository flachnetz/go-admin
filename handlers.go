package admin

import (
	"fmt"
	"github.com/goji/httpauth"
	"github.com/kardianos/osext"
	"io"
	"io/ioutil"
	"net/http"
	pprofH "net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"time"
)

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
				Hostname:   hostname,
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
			w.Header().Set("Content-Disposition", "attachment; filename="+filename)

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

func RequireAuth(user, pass string, configs ...RouteConfig) RouteConfig {
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
