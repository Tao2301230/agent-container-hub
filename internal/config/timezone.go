package config

import (
	"path/filepath"
	"strings"
)

func resolveDisplayTimezone(displayEnv, tzEnv string) string {
	if displayEnv != "" {
		return displayEnv
	}
	if tzEnv != "" {
		return tzEnv
	}
	if z := inferIANAFromLocalZoneFile(); z != "" {
		return z
	}
	return "UTC"
}

func inferIANAFromLocalZoneFile() string {
	for _, p := range []string{
		"/etc/localtime",
		"/var/db/timezone/localtime",
	} {
		if name := ianaFromZoneinfoPath(p); name != "" {
			return name
		}
	}
	return ""
}

func ianaFromZoneinfoPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}
	resolved = filepath.ToSlash(resolved)
	for _, marker := range []string{"/zoneinfo/", "/zoneinfo.default/"} {
		if i := strings.LastIndex(resolved, marker); i >= 0 {
			zone := resolved[i+len(marker):]
			zone = strings.TrimPrefix(zone, "posix/")
			zone = strings.TrimPrefix(zone, "right/")
			return zone
		}
	}
	return ""
}
