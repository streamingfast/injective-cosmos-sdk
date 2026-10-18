//go:build tools

// Package tools pins the consensuswarn build used by
// .github/workflows/consensuswarn.yml. Upstream only ships a Docker action
// built with Go 1.23 and golang.org/x/tools v0.20, which can't load this
// repository's Go version; golang.org/x/tools is raised here instead.
package tools

import _ "github.com/orijtech/consensuswarn"
