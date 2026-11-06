// Package catalog embeds the signed-version software catalog used by WinDeploy.
package catalog

import (
	"embed"
	"io"
	"sync"

	"windeploy/core/packages"
)

//go:embed packages.yaml
var Files embed.FS

var (
	once    sync.Once
	loaded  packages.Catalog
	loadErr error
)

// Load returns the embedded, validated catalog. Parsing happens once per process.
func Load() (packages.Catalog, error) {
	once.Do(func() {
		file, err := Files.Open("packages.yaml")
		if err != nil {
			loadErr = err
			return
		}
		defer file.Close()
		loaded, loadErr = packages.LoadCatalog(io.Reader(file))
	})
	return loaded, loadErr
}
