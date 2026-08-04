package version

import (
	"fmt"
)

var (
	version = "vUnknown"
	source  = "Unknown"
)

func Format() string {
	return fmt.Sprintf("Running Poll-Bot %s", version)
}
func Version() string {
	return version
}
func Source() string {
	return source
}
