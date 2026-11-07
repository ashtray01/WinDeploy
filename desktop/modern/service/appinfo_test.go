package service

import "testing"

func TestAppInfoUsesBuildIdentity(t *testing.T) {
	info := (&Service{}).GetAppInfo()
	if info.Name != "WinDeploy" || info.Channel != "public-beta" {
		t.Fatalf("unexpected app identity: %+v", info)
	}
	if info.Version == "" || info.Commit == "" {
		t.Fatalf("version and commit must not be empty: %+v", info)
	}
}
