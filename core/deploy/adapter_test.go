package deploy

import (
	"context"
	"errors"
	"testing"
	"time"

	"windeploy/core/engine"
	"windeploy/core/inventory"
	"windeploy/core/network"
	"windeploy/core/shares"
)

type fakeConnector struct {
	connections []shares.Connection
	requests    int
}

func (c *fakeConnector) Connect(_ context.Context, request shares.ConnectionRequest) error {
	c.requests++
	c.connections = append(c.connections, shares.Connection{RemoteName: request.RemoteName})
	return nil
}
func (c *fakeConnector) List(context.Context) ([]shares.Connection, error) {
	return append([]shares.Connection{}, c.connections...), nil
}
func (c *fakeConnector) Disconnect(context.Context, string, bool) error { return nil }

type fakeShareStore struct{}

func (fakeShareStore) Save(context.Context, string, string, string) error { return nil }
func (fakeShareStore) Read(context.Context, string) (shares.Credential, bool, error) {
	return shares.Credential{}, false, nil
}
func (fakeShareStore) Delete(context.Context, string) error { return nil }

type fakeShortcut struct{}

func (fakeShortcut) Create(context.Context, shares.ShortcutSpec) error { return nil }

type fakeResolver struct{ err error }

func (r fakeResolver) LookupHost(context.Context, string) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return []string{"127.0.0.1"}, nil
}

type fakePinger struct{}

func (fakePinger) Ping(context.Context, string) (time.Duration, error) { return time.Millisecond, nil }

func shareAdapter() (*Adapter, *fakeConnector) {
	connector := &fakeConnector{}
	manager := &shares.Manager{Store: fakeShareStore{}, Connector: connector, Shortcuts: fakeShortcut{}}
	return &Adapter{Shares: manager, ShortcutDir: `C:\Desktop`}, connector
}

func TestRebootVerifyAndComputerNameReadback(t *testing.T) {
	task := engine.Task{ID: "rename", Name: "Rename", Category: "windows", Parameters: map[string]string{"setting": "computer-name", "name": "PC-NEW"}}
	adapter := &Adapter{System: inventory.SystemInfo{Hostname: "PC-OLD"}}
	adapter.markReboot(task.ID)
	if err := adapter.Verify(context.Background(), task); !errors.Is(err, engine.ErrRebootPending) {
		t.Fatalf("false verified reboot: %v", err)
	}
	fresh := &Adapter{System: inventory.SystemInfo{Hostname: "PC-OLD"}}
	if err := fresh.Verify(context.Background(), task); err == nil {
		t.Fatal("rename verified with old hostname")
	}
	fresh.System.Hostname = "pc-new"
	if err := fresh.Verify(context.Background(), task); err != nil {
		t.Fatal(err)
	}
}

func TestShareComponentLifecycle(t *testing.T) {
	adapter, connector := shareAdapter()
	task := engine.Task{ID: "finance", Name: "Финансы", Category: "shares", Status: engine.Pending, Parameters: map[string]string{"unc": `\\fileserver\finance`, "shortcut": "true"}}
	status, err := adapter.Precheck(context.Background(), task)
	if err != nil || status != engine.Pending {
		t.Fatalf("precheck: %s %v", status, err)
	}
	if reboot, err := adapter.Execute(context.Background(), task); err != nil || reboot {
		t.Fatalf("execute: %v %v", reboot, err)
	}
	if err := adapter.Verify(context.Background(), task); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if connector.requests != 1 {
		t.Fatalf("unexpected connect calls: %d", connector.requests)
	}
	status, err = adapter.Precheck(context.Background(), task)
	if err != nil || status != engine.AlreadyInstalled {
		t.Fatalf("second precheck: %s %v", status, err)
	}
}

func TestShareComponentValidation(t *testing.T) {
	adapter, _ := shareAdapter()
	task := engine.Task{ID: "bad", Category: "shares", Status: engine.Pending, Parameters: map[string]string{}}
	if _, err := adapter.Precheck(context.Background(), task); err == nil {
		t.Fatal("share without UNC accepted")
	}
}

func TestDiagnosticsComponent(t *testing.T) {
	adapter := &Adapter{
		NetworkConfig: func(context.Context) ([]inventory.AdapterConfig, error) {
			return []inventory.AdapterConfig{{Index: 1, Name: "Ethernet", Operational: true, IPv4Enabled: true, DHCPv4Enabled: false, Gateways: []string{"10.0.0.1"}}}, nil
		},
		NetworkDeps: network.Dependencies{Resolver: fakeResolver{}, Pinger: fakePinger{}},
	}
	task := engine.Task{ID: "network", Category: "diagnostics", Status: engine.Pending, Parameters: map[string]string{"dns": "localhost"}}
	if status, err := adapter.Precheck(context.Background(), task); err != nil || status != engine.Pending {
		t.Fatalf("precheck: %s %v", status, err)
	}
	if _, err := adapter.Execute(context.Background(), task); err != nil {
		t.Fatalf("diagnostics failed: %v", err)
	}
	adapter.NetworkDeps.Resolver = fakeResolver{err: errors.New("dns down")}
	if _, err := adapter.Execute(context.Background(), task); err == nil {
		t.Fatal("failed diagnostics were reported as success")
	}
}

func TestUnsupportedCategory(t *testing.T) {
	adapter := &Adapter{}
	task := engine.Task{ID: "x", Category: "unknown", Status: engine.Pending}
	if _, err := adapter.Precheck(context.Background(), task); err == nil {
		t.Fatal("unsupported category accepted")
	}
	if _, err := adapter.Execute(context.Background(), task); err == nil {
		t.Fatal("unsupported category executed")
	}
}

func TestInvalidComponentParameters(t *testing.T) {
	adapter := &Adapter{}
	if _, err := adapter.Execute(context.Background(), engine.Task{ID: "p", Category: "packages", Parameters: map[string]string{}}); err == nil {
		t.Fatal("package without package_ref accepted")
	}
	if _, err := adapter.Execute(context.Background(), engine.Task{ID: "d", Category: "drivers", Parameters: map[string]string{}}); err == nil {
		t.Fatal("driver without ids accepted")
	}
	if _, err := adapter.Execute(context.Background(), engine.Task{ID: "b", Category: "browsers", Parameters: map[string]string{"browser": "opera"}}); err == nil {
		t.Fatal("unsupported browser accepted")
	}
}
