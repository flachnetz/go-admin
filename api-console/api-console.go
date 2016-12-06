package apiconsole

import (
	"github.com/flachnetz/go-admin"
	"net/http"
)

func WithApiConsole(ramlContent string) admin.RouteConfig {
	fsHandler := http.StripPrefix("/api-console", http.FileServer(assetFS()))

	return admin.
		WithGetHandlerFunc("/api-console", func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/api-console" {
				localRedirect(w, req, "./api-console/main.html")
			}

			if req.URL.Path == "/api-console/api.raml" {
				w.Write([]byte(ramlContent))
				return
			}

			fsHandler.ServeHTTP(w, req)
		}).
		Describe("Interactive api console of the services api.").
		Wildcard(true)
}

// localRedirect gives StatusTemporaryRedirect response.
// It does not convert relative paths to absolute paths like Redirect does.
func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}

	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
