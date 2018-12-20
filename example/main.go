package main

import (
	"net/http"

	. "github.com/flachnetz/go-admin"
	"github.com/pkg/browser"
	"time"
)

func main() {
	admin := NewAdminHandler("example", "/admin",
		WithDefaults(),
		WithBuildInfo(BuildInfo{}),
		WithMetrics(nil),

		// WithGeneric("/service/stats", cache.Stats),
		// WithGeneric("/services", discovery.Services).Description("List of services"),

		// WithGetHandler("/reboot", RebootHandler())

		RequireAuth("admin", "secret",
			WithForceGC(),
			WithHeapDump(),
			WithPProfHandlers(),
		))

	go func() {
		time.Sleep(1 * time.Second)
		browser.OpenURL("http://localhost:5000/admin")
	}()

	http.ListenAndServe(":5000", admin)
}
