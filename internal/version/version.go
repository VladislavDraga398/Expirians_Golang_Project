package version

import "fmt"

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Info returns version information populated via -ldflags.
func Info() (v, c, d string) { return version, commit, date }

// GetVersion returns the version string
func GetVersion() string {
	return version
}

// GetCommit returns the commit hash
func GetCommit() string {
	return commit
}

// GetDate returns the build date
func GetDate() string {
	return date
}

func String() string {
	return fmt.Sprintf("version=%s commit=%s date=%s", version, commit, date)
}
