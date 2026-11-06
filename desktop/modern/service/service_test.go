package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"windeploy/core/inventory"
)

type fixtureProvider struct {
	gate    chan struct{}
	entered chan struct{}
	partial bool
	calls   atomic.Int32
}

func (p *fixtureProvider) System(ctx context.Context) (inventory.SystemInfo, error) {
	p.calls.Add(1)
	if p.entered != nil {
		close(p.entered)
	}
	if p.gate != nil {
		select {
		case <-p.gate:
		case <-ctx.Done():
			return inventory.SystemInfo{}, ctx.Err()
		}
	}
	return inventory.SystemInfo{Hostname: "fixture", Architecture: "amd64", Windows: inventory.Version{Major: 10, Build: 26100, ProductType: 1}}, nil
}
func (p *fixtureProvider) CPU(context.Context) (inventory.CPUInfo, error) {
	if p.partial {
		return inventory.CPUInfo{}, errors.New("private-provider-diagnostic")
	}
	return inventory.CPUInfo{Model: "fixture", LogicalProcessors: 4}, nil
}
func (p *fixtureProvider) Memory(context.Context) (inventory.MemoryInfo, error) {
	return inventory.MemoryInfo{TotalPhysicalBytes: 8 << 30}, nil
}
func (p *fixtureProvider) Volumes(context.Context) ([]inventory.Volume, error) {
	return []inventory.Volume{}, nil
}
func (p *fixtureProvider) Interfaces(context.Context) ([]inventory.InterfaceInfo, error) {
	return []inventory.InterfaceInfo{}, nil
}
func (p *fixtureProvider) NetworkConfig(context.Context) ([]inventory.AdapterConfig, error) {
	return []inventory.AdapterConfig{}, nil
}
func (p *fixtureProvider) Apps(context.Context) ([]inventory.InstalledApp, error) {
	return []inventory.InstalledApp{}, nil
}

func TestScanPartialAndJournal(t *testing.T) {
	for _, partial := range []bool{false, true} {
		s := New(&fixtureProvider{partial: partial})
		defer s.ServiceShutdown()
		if state := s.GetState(); state.Report != nil || state.Generation != 0 {
			t.Fatal("invented initial inventory")
		}
		if _, err := s.StartScan(); err != nil {
			t.Fatal(err)
		}
		s.wg.Wait()
		state := s.GetState()
		want := "complete"
		if partial {
			want = "partial"
		}
		if state.Scanning || state.Status != want || state.Report == nil || state.Report.Complete == partial {
			t.Fatalf("status=%s", state.Status)
		}
		data, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "private-provider-diagnostic") {
			t.Fatal("raw diagnostic exposed")
		}
		if len(s.GetJournal()) < 9 {
			t.Fatal("real source messages missing")
		}
	}
}

func TestCancellationDoesNotOverlapAndShutdownStopsWork(t *testing.T) {
	p := &fixtureProvider{gate: make(chan struct{}), entered: make(chan struct{})}
	s := New(p)
	first, err := s.StartScan()
	if err != nil {
		t.Fatal(err)
	}
	<-p.entered
	second, err := s.StartScan()
	if err != nil || first.Generation != second.Generation || p.calls.Load() != 1 {
		t.Fatal("overlapping scan")
	}
	if !s.CancelScan().CancelRequested {
		t.Fatal("cancel request lost")
	}
	s.wg.Wait()
	if s.GetState().Status != "cancelled" {
		t.Fatal("cancelled scan reported success")
	}
	s.ServiceShutdown()
	if _, err = s.StartScan(); err == nil {
		t.Fatal("started after shutdown")
	}
}
