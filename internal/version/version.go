package version

import (
	"fmt"
	"runtime/debug"
)

func Get() string {
	var (
		modified bool
		revision string
	)

	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value[:7]

			case "vcs.modified":
				if setting.Value == "true" {
					modified = true
				}
			}
		}
	}

	if revision == "" {
		return "unavailable"
	}

	if modified {
		return fmt.Sprintf("%s+dirty", revision)
	}

	return revision
}
