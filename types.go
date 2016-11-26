package admin

type BuildInfo struct {
	Version   string `json:",omitempty"`
	GitHash   string `json:",omitempty"`
	BuildTime string `json:",omitempty"`
}

// We actually dont use any of those methods, but we want some kind of
// type safety for the WithMetrics() method, without depending on the
// metrics package.
type MetricsRegistry interface {
	GetOrRegister(string, interface{}) interface{}
	Register(string, interface{}) error
	RunHealthchecks()
	UnregisterAll()
}
