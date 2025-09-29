package version

import "fmt"

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Info returns version information populated via -ldflags.
func Info() (v, c, d string) { return version, commit, date }

func String() string {
	return fmt.Sprintf("version=%s commit=%s date=%s", version, commit, date)
}
