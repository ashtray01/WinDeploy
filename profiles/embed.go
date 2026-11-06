// Package profileassets provides the existing example profiles for portable viewers.
package profileassets

import "embed"

//go:embed *.yaml
var Files embed.FS
