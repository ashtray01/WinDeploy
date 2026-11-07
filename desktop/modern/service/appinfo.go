package service

import "windeploy/internal/buildinfo"

// AppInfo is the release identity exposed to the desktop UI. It contains no
// machine, user or session data and is safe to show in screenshots.
type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Channel string `json:"channel"`
}

// GetAppInfo returns the version injected at build time. The UI must not keep a
// second hard-coded version number.
func (s *Service) GetAppInfo() AppInfo {
	return AppInfo{
		Name:    buildinfo.ProductName,
		Version: buildinfo.DisplayVersion(),
		Commit:  buildinfo.DisplayCommit(),
		Channel: "public-beta",
	}
}
