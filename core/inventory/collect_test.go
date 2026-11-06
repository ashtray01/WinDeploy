package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeProvider struct {
	calls  []string
	fail   map[string]error
	cancel context.CancelFunc
}

func (p *fakeProvider) read(section string) error {
	p.calls = append(p.calls, section)
	if section == "cpu" && p.cancel != nil {
		p.cancel()
	}
	return p.fail[section]
}
func (p *fakeProvider) System(context.Context) (SystemInfo, error) {
	return SystemInfo{Hostname: "test", Architecture: "amd64"}, p.read("system")
}
func (p *fakeProvider) CPU(context.Context) (CPUInfo, error) {
	return CPUInfo{LogicalProcessors: 2}, p.read("cpu")
}
func (p *fakeProvider) Memory(context.Context) (MemoryInfo, error) {
	return MemoryInfo{TotalPhysicalBytes: 8 << 30}, p.read("memory")
}
func (p *fakeProvider) Volumes(context.Context) ([]Volume, error) {
	return []Volume{}, p.read("volumes")
}
func (p *fakeProvider) Interfaces(context.Context) ([]InterfaceInfo, error) {
	return []InterfaceInfo{}, p.read("interfaces")
}
func (p *fakeProvider) NetworkConfig(context.Context) ([]AdapterConfig, error) {
	return []AdapterConfig{}, p.read("network")
}
func (p *fakeProvider) Apps(context.Context) ([]InstalledApp, error) {
	return []InstalledApp{}, p.read("apps")
}

func TestCollectCompleteAndPartial(t *testing.T) {
	p := &fakeProvider{}
	r, err := Collect(context.Background(), p)
	if err != nil || !r.Complete || len(r.Sections) != 7 || len(p.calls) != 7 || r.System == nil || r.FinishedAt.Before(r.StartedAt) {
		t.Fatalf("bad complete report: %v", err)
	}
	p = &fakeProvider{fail: map[string]error{"cpu": errors.New("password=private"), "apps": errors.New("password=private")}}
	r, err = Collect(context.Background(), p)
	if !errors.Is(err, ErrIncomplete) || r.Complete || r.Sections[1].Status != "partial" || r.Sections[6].Status != "failed" || len(p.calls) != 7 {
		t.Fatalf("partial status lost: %+v %v", r.Sections, err)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "password") || strings.Contains(string(data), "private") {
		t.Fatal("raw error leaked")
	}
}

func TestCollectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &fakeProvider{cancel: cancel}
	r, err := Collect(ctx, p)
	if !errors.Is(err, context.Canceled) || r.Complete || len(p.calls) != 2 || r.Sections[1].Status != "cancelled" || r.Sections[2].Status != "not-collected" {
		t.Fatalf("%+v %v", r.Sections, err)
	}
	if _, err = Collect(context.Background(), nil); err == nil {
		t.Fatal("nil provider accepted")
	}
}
