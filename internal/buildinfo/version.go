// Package buildinfo exposes build-time metadata for Memora services.
package buildinfo

// BuildInfo describes the build that produced a Memora service binary.
type BuildInfo struct {
	Service string
	Version string
	Commit  string
	BuiltAt string
}

var (
	Version = "dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)

// Info returns build metadata shared by all Memora services.
func Info() BuildInfo {
	return BuildInfo{
		Service: "memora",
		Version: Version,
		Commit:  Commit,
		BuiltAt: BuiltAt,
	}
}
