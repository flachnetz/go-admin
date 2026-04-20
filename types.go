package admin

type BuildInfo struct {
	Version   string `json:",omitempty"`
	GitHash   string `json:",omitempty"`
	BuildTime string `json:",omitempty"`
}
