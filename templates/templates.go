package templates

import (
	"embed"
	"regexp"
)

// FS contém todos os templates embutidos no binário final.
// Permite que o antigravity-operator funcione de forma 100% autônoma (zero runtime dependencies).
//
//go:embed rules/* session/* mcps/* skills/*
var FS embed.FS

// pinnedNpmPackage matches name@X.Y.Z, scoped or not. npx -y runs whatever the
// registry serves for an unpinned or unclaimed name, so every MCP server run
// through npx must name an exact version.
var pinnedNpmPackage = regexp.MustCompile(`^(@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*@\d+\.\d+\.\d+$`)

// IsPinnedNpmPackage reports whether pkg names an exact npm version.
func IsPinnedNpmPackage(pkg string) bool {
	return pinnedNpmPackage.MatchString(pkg)
}

// NpxPackage returns the package an npx invocation runs: the first argument
// that is not a flag, or "" when there is none.
func NpxPackage(args []string) string {
	for _, a := range args {
		if a != "" && a[0] != '-' {
			return a
		}
	}
	return ""
}
