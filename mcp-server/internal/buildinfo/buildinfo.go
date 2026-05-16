package buildinfo

var (
	Version = "0.1.0-dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func String(name string) string {
	if Commit == "unknown" && Date == "unknown" {
		return name + " " + Version
	}
	return name + " " + Version + " (" + Commit + ", " + Date + ")"
}
