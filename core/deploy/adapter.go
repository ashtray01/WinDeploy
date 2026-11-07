package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"windeploy/core/browsers"
	"windeploy/core/drivers"
	"windeploy/core/engine"
	"windeploy/core/inventory"
	"windeploy/core/network"
	"windeploy/core/packages"
	"windeploy/core/printers"
	"windeploy/core/settings"
	"windeploy/core/shares"
)

// Adapter implements engine.Adapter for profile components. It dispatches by
// Component category and never reads credentials from task parameters.
type Adapter struct {
	PackageManager   *packages.Manager
	Shares           *shares.Manager
	Drivers          *drivers.Manager
	Printers         *printers.Manager
	Browsers         *browsers.Manager
	Settings         *settings.Manager
	NetworkDeps      network.Dependencies
	NetworkConfig    func(context.Context) ([]inventory.AdapterConfig, error)
	ShortcutDir      string
	System           inventory.SystemInfo
	CheckUserContext func(context.Context) error

	rebootMu sync.Mutex
	reboot   map[string]bool
}

func (a *Adapter) markReboot(taskID string) {
	a.rebootMu.Lock()
	defer a.rebootMu.Unlock()
	if a.reboot == nil {
		a.reboot = map[string]bool{}
	}
	a.reboot[taskID] = true
}

func (a *Adapter) rebootPending(taskID string) bool {
	a.rebootMu.Lock()
	defer a.rebootMu.Unlock()
	return a.reboot[taskID]
}

// Precheck classifies the component from read-only evidence. It never mutates.
func (a *Adapter) Precheck(ctx context.Context, task engine.Task) (engine.TaskStatus, error) {
	if err := ctx.Err(); err != nil {
		return engine.Pending, err
	}
	if RequiresUserContext(task) {
		if err := checkUserContext(ctx, a.CheckUserContext); err != nil {
			return engine.Pending, err
		}
	}
	switch task.Category {
	case "packages":
		return a.precheckPackage(ctx, task)
	case "shares":
		return a.precheckShare(ctx, task)
	case "drivers":
		return a.precheckDriver(ctx, task)
	case "printers":
		return a.precheckPrinter(ctx, task)
	case "browsers", "windows", "diagnostics":
		return engine.Pending, nil
	default:
		return engine.Pending, fmt.Errorf("unsupported component category")
	}
}

// Execute performs the operation and records any reboot obligation for Verify.
func (a *Adapter) Execute(ctx context.Context, task engine.Task) (bool, error) {
	if RequiresUserContext(task) {
		if err := checkUserContext(ctx, a.CheckUserContext); err != nil {
			return false, err
		}
	}
	reboot, err := a.execute(ctx, task)
	if reboot {
		a.markReboot(task.ID)
	}
	return reboot, err
}

func (a *Adapter) execute(ctx context.Context, task engine.Task) (bool, error) {
	switch task.Category {
	case "packages":
		return a.executePackage(ctx, task)
	case "shares":
		return false, a.executeShare(ctx, task)
	case "drivers":
		return a.executeDriver(ctx, task)
	case "printers":
		return false, a.executePrinter(ctx, task)
	case "browsers":
		return false, a.executeBrowser(ctx, task)
	case "windows":
		return a.executeWindows(ctx, task)
	case "diagnostics":
		return false, a.executeDiagnostics(ctx, task)
	default:
		return false, fmt.Errorf("unsupported component category")
	}
}

// Verify reports a persistent barrier for operations requiring reboot. A fresh
// adapter after a confirmed boot change performs the real category read-back.
func (a *Adapter) Verify(ctx context.Context, task engine.Task) error {
	if RequiresUserContext(task) {
		if err := checkUserContext(ctx, a.CheckUserContext); err != nil {
			return err
		}
	}
	if a.rebootPending(task.ID) {
		return engine.ErrRebootPending
	}
	switch task.Category {
	case "packages":
		return a.verifyPackage(ctx, task)
	case "shares":
		return a.verifyShare(ctx, task)
	case "drivers":
		return a.verifyDriver(ctx, task)
	case "printers":
		return a.verifyPrinter(ctx, task)
	case "windows":
		if strings.EqualFold(strings.TrimSpace(task.Parameters["setting"]), "computer-name") {
			if a.System.Hostname == "" || !strings.EqualFold(a.System.Hostname, strings.TrimSpace(task.Parameters["name"])) {
				return fmt.Errorf("active computer name was not confirmed")
			}
		}
		return nil
	case "browsers", "diagnostics":
		return nil
	default:
		return fmt.Errorf("unsupported component category")
	}
}

// --- packages ---

func (a *Adapter) packageRef(task engine.Task) (string, error) {
	ref := strings.TrimSpace(task.Parameters["package_ref"])
	if ref == "" {
		return "", fmt.Errorf("package component requires package_ref")
	}
	return ref, nil
}

func (a *Adapter) precheckPackage(ctx context.Context, task engine.Task) (engine.TaskStatus, error) {
	if a.PackageManager == nil {
		return engine.Pending, fmt.Errorf("package manager is unavailable")
	}
	ref, err := a.packageRef(task)
	if err != nil {
		return engine.Pending, err
	}
	planner := packages.Planner{Catalog: a.PackageManager.Catalog, Facts: a.PackageManager.Facts, System: a.PackageManager.System, Winget: a.PackageManager.Winget}
	plan, err := planner.Build(ctx, []string{ref})
	if err != nil || len(plan.Items) != 1 {
		return engine.Pending, fmt.Errorf("package plan is invalid")
	}
	switch plan.Items[0].Action {
	case packages.ActionUpToDate:
		return engine.AlreadyInstalled, nil
	case packages.ActionIncompatible:
		return engine.NotApplicable, nil
	case packages.ActionInstall, packages.ActionUpdate:
		return engine.Pending, nil
	default:
		return engine.Pending, fmt.Errorf("package is unavailable")
	}
}

func (a *Adapter) executePackage(ctx context.Context, task engine.Task) (bool, error) {
	if a.PackageManager == nil {
		return false, fmt.Errorf("package manager is unavailable")
	}
	ref, err := a.packageRef(task)
	if err != nil {
		return false, err
	}
	run, err := a.PackageManager.Install(ctx, []string{ref}, packages.InstallOptions{MaxAttempts: 2})
	if err != nil {
		return false, err
	}
	for _, result := range run.Results {
		if result.PackageID != ref {
			continue
		}
		switch result.Status {
		case packages.StatusSuccess, packages.StatusRebootRequired, packages.StatusUpToDate:
			if result.RebootRequired {
				a.markReboot(task.ID)
			}
			return result.RebootRequired, nil
		case packages.StatusCancelled:
			return false, context.Canceled
		default:
			return false, fmt.Errorf("package operation failed")
		}
	}
	return false, fmt.Errorf("package was not processed")
}

func (a *Adapter) verifyPackage(ctx context.Context, task engine.Task) error {
	if a.PackageManager == nil {
		return fmt.Errorf("package manager is unavailable")
	}
	ref, err := a.packageRef(task)
	if err != nil {
		return err
	}
	pkg, ok := a.PackageManager.Catalog.Find(ref)
	if !ok {
		return fmt.Errorf("package is missing from the catalog")
	}
	state, detectErr := packages.Detect(ctx, a.PackageManager.Facts, pkg)
	if detectErr != nil || !state.Installed {
		return fmt.Errorf("installed package was not confirmed")
	}
	return nil
}

// --- shares ---

func shareFromTask(task engine.Task) (shares.Share, error) {
	unc := strings.TrimSpace(task.Parameters["unc"])
	if unc == "" {
		return shares.Share{}, fmt.Errorf("share component requires unc")
	}
	share := shares.Share{
		ID: task.ID, UNC: unc, Name: task.Parameters["name"], Username: task.Parameters["username"],
		Persistent: parseBool(task.Parameters["persistent"]), Shortcut: parseBool(task.Parameters["shortcut"]),
	}
	if err := share.Validate(); err != nil {
		return shares.Share{}, err
	}
	return share, nil
}

func (a *Adapter) precheckShare(ctx context.Context, task engine.Task) (engine.TaskStatus, error) {
	if a.Shares == nil || a.Shares.Connector == nil {
		return engine.Pending, fmt.Errorf("share manager is unavailable")
	}
	share, err := shareFromTask(task)
	if err != nil {
		return engine.Pending, err
	}
	connections, err := a.Shares.Connector.List(ctx)
	if err != nil {
		return engine.Pending, fmt.Errorf("existing connections are unavailable")
	}
	for _, connection := range connections {
		if strings.EqualFold(strings.TrimRight(connection.RemoteName, `\`), strings.TrimRight(share.UNC, `\`)) {
			return engine.AlreadyInstalled, nil
		}
	}
	return engine.Pending, nil
}

func (a *Adapter) executeShare(ctx context.Context, task engine.Task) error {
	if a.Shares == nil {
		return fmt.Errorf("share manager is unavailable")
	}
	share, err := shareFromTask(task)
	if err != nil {
		return err
	}
	_, err = a.Shares.Add(ctx, share, shares.Options{ShortcutDir: a.ShortcutDir})
	if err != nil {
		return err
	}
	return nil
}

func (a *Adapter) verifyShare(ctx context.Context, task engine.Task) error {
	if a.Shares == nil || a.Shares.Connector == nil {
		return fmt.Errorf("share manager is unavailable")
	}
	share, err := shareFromTask(task)
	if err != nil {
		return err
	}
	connections, err := a.Shares.Connector.List(ctx)
	if err != nil {
		return fmt.Errorf("connections could not be confirmed")
	}
	for _, connection := range connections {
		if strings.EqualFold(strings.TrimRight(connection.RemoteName, `\`), strings.TrimRight(share.UNC, `\`)) {
			return nil
		}
	}
	return fmt.Errorf("share connection was not confirmed")
}

// --- drivers ---

func driverRequest(task engine.Task) (drivers.InstallRequest, error) {
	infPath := strings.TrimSpace(task.Parameters["inf_path"])
	hardwareID := strings.TrimSpace(task.Parameters["hardware_id"])
	if infPath == "" || hardwareID == "" {
		return drivers.InstallRequest{}, fmt.Errorf("driver component requires inf_path and hardware_id")
	}
	return drivers.InstallRequest{INFPath: infPath, HardwareID: hardwareID}, nil
}

func (a *Adapter) precheckDriver(ctx context.Context, task engine.Task) (engine.TaskStatus, error) {
	if a.Drivers == nil || a.Drivers.Devices == nil {
		return engine.Pending, fmt.Errorf("driver manager is unavailable")
	}
	request, err := driverRequest(task)
	if err != nil {
		return engine.Pending, err
	}
	devices, err := a.Drivers.Devices.Devices(ctx)
	if err != nil {
		return engine.Pending, fmt.Errorf("devices are unavailable")
	}
	for _, device := range devices {
		if hasHardwareID(device, request.HardwareID) && device.Status == "ok" {
			return engine.AlreadyInstalled, nil
		}
	}
	return engine.Pending, nil
}

func (a *Adapter) executeDriver(ctx context.Context, task engine.Task) (bool, error) {
	if a.Drivers == nil {
		return false, fmt.Errorf("driver manager is unavailable")
	}
	request, err := driverRequest(task)
	if err != nil {
		return false, err
	}
	result, err := a.Drivers.Install(ctx, request)
	if err != nil {
		return false, err
	}
	if result.RebootRequired {
		a.markReboot(task.ID)
	}
	return result.RebootRequired, nil
}

func (a *Adapter) verifyDriver(ctx context.Context, task engine.Task) error {
	if a.Drivers == nil || a.Drivers.Devices == nil {
		return fmt.Errorf("driver manager is unavailable")
	}
	request, err := driverRequest(task)
	if err != nil {
		return err
	}
	devices, err := a.Drivers.Devices.Devices(ctx)
	if err != nil {
		return fmt.Errorf("devices are unavailable")
	}
	for _, device := range devices {
		if hasHardwareID(device, request.HardwareID) && device.Status == "ok" {
			return nil
		}
	}
	return fmt.Errorf("driver installation was not confirmed")
}

func hasHardwareID(device drivers.Device, hardwareID string) bool {
	for _, id := range device.HardwareIDs {
		if strings.EqualFold(id, hardwareID) {
			return true
		}
	}
	return false
}

// --- printers ---

func printerRequest(task engine.Task) (printers.AddRequest, error) {
	request := printers.AddRequest{
		Name: strings.TrimSpace(task.Parameters["name"]), INFPath: strings.TrimSpace(task.Parameters["inf_path"]),
		Model: strings.TrimSpace(task.Parameters["model"]), IPAddress: strings.TrimSpace(task.Parameters["ip_address"]),
		Share: parseBool(task.Parameters["share"]), ShareName: task.Parameters["share_name"],
		SetDefault: parseBool(task.Parameters["set_default"]),
	}
	if err := printers.ValidateAddRequest(request); err != nil {
		return printers.AddRequest{}, err
	}
	return request, nil
}

func (a *Adapter) precheckPrinter(ctx context.Context, task engine.Task) (engine.TaskStatus, error) {
	if a.Printers == nil || a.Printers.Printers == nil {
		return engine.Pending, fmt.Errorf("printer manager is unavailable")
	}
	request, err := printerRequest(task)
	if err != nil {
		return engine.Pending, err
	}
	items, err := a.Printers.Printers.Printers(ctx)
	if err != nil {
		return engine.Pending, fmt.Errorf("printers are unavailable")
	}
	for _, printer := range items {
		if strings.EqualFold(printer.Name, request.Name) {
			return engine.AlreadyInstalled, nil
		}
	}
	return engine.Pending, nil
}

func (a *Adapter) executePrinter(ctx context.Context, task engine.Task) error {
	if a.Printers == nil {
		return fmt.Errorf("printer manager is unavailable")
	}
	request, err := printerRequest(task)
	if err != nil {
		return err
	}
	_, err = a.Printers.Add(ctx, request)
	return err
}

func (a *Adapter) verifyPrinter(ctx context.Context, task engine.Task) error {
	if a.Printers == nil || a.Printers.Printers == nil {
		return fmt.Errorf("printer manager is unavailable")
	}
	request, err := printerRequest(task)
	if err != nil {
		return err
	}
	items, err := a.Printers.Printers.Printers(ctx)
	if err != nil {
		return fmt.Errorf("printers are unavailable")
	}
	for _, printer := range items {
		if strings.EqualFold(printer.Name, request.Name) {
			return nil
		}
	}
	return fmt.Errorf("printer was not confirmed")
}

// --- browsers ---

func browserPolicy(task engine.Task) (browsers.Policy, error) {
	policy := browsers.Policy{
		Browser:  browsers.Browser(strings.ToLower(task.Parameters["browser"])),
		Scope:    browsers.Scope(strings.ToLower(task.Parameters["scope"])),
		Homepage: task.Parameters["homepage"],
	}
	if policy.Scope == "" {
		policy.Scope = browsers.ScopeUser
	}
	for _, line := range strings.Split(task.Parameters["startup_urls"], "\n") {
		if strings.TrimSpace(line) != "" {
			policy.StartupURLs = append(policy.StartupURLs, strings.TrimSpace(line))
		}
	}
	if raw := task.Parameters["bookmarks_json"]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &policy.Bookmarks); err != nil {
			return browsers.Policy{}, fmt.Errorf("invalid bookmarks_json")
		}
	}
	if raw := task.Parameters["webapps_json"]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &policy.WebApps); err != nil {
			return browsers.Policy{}, fmt.Errorf("invalid webapps_json")
		}
	}
	if err := browsers.Validate(policy); err != nil {
		return browsers.Policy{}, err
	}
	return policy, nil
}

func (a *Adapter) executeBrowser(ctx context.Context, task engine.Task) error {
	if a.Browsers == nil {
		return fmt.Errorf("browser manager is unavailable")
	}
	policy, err := browserPolicy(task)
	if err != nil {
		return err
	}
	_, err = a.Browsers.Apply(ctx, policy, false)
	return err
}

// --- windows settings ---

func (a *Adapter) executeWindows(ctx context.Context, task engine.Task) (bool, error) {
	if a.Settings == nil {
		return false, fmt.Errorf("settings manager is unavailable")
	}
	switch strings.ToLower(strings.TrimSpace(task.Parameters["setting"])) {
	case "computer-name":
		result, err := a.Settings.SetComputerName(ctx, task.Parameters["name"], false)
		return result.RebootRequired, err
	case "timezone":
		_, err := a.Settings.SetTimezone(ctx, task.Parameters["timezone"], false)
		return false, err
	case "power-plan":
		_, err := a.Settings.SetPowerPlan(ctx, task.Parameters["guid"], false)
		return false, err
	case "explorer":
		showHidden, err := optionalBool(task.Parameters, "show_hidden")
		if err != nil {
			return false, err
		}
		showExtensions, err := optionalBool(task.Parameters, "show_extensions")
		if err != nil {
			return false, err
		}
		_, err = a.Settings.ApplyExplorer(ctx, settings.ExplorerOptions{ShowHidden: showHidden, ShowExtensions: showExtensions}, false)
		return false, err
	default:
		return false, fmt.Errorf("unsupported windows setting")
	}
}

// --- diagnostics ---

func (a *Adapter) executeDiagnostics(ctx context.Context, task engine.Task) error {
	if a.NetworkConfig == nil {
		return fmt.Errorf("network configuration is unavailable")
	}
	adapters, err := a.NetworkConfig(ctx)
	if err != nil {
		return fmt.Errorf("network adapters are unavailable")
	}
	plan := network.Plan{Timeout: 5 * time.Second}
	if parseBoolDefault(task.Parameters["dhcp"], true) {
		plan.Adapters = adapters
	}
	if parseBoolDefault(task.Parameters["gateways"], true) {
		for _, gateway := range gatherGateways(adapters) {
			plan.Gateways = append(plan.Gateways, gateway)
		}
	}
	if dnsName := strings.TrimSpace(task.Parameters["dns"]); dnsName != "" {
		plan.DNSNames = append(plan.DNSNames, dnsName)
		if parseBoolDefault(task.Parameters["dns_servers"], false) {
			for _, adapter := range adapters {
				if adapter.Operational {
					plan.Servers = append(plan.Servers, adapter.DNSServers...)
				}
			}
		}
	}
	for _, host := range splitTargets(task.Parameters["icmp"]) {
		plan.ICMPHosts = append(plan.ICMPHosts, host)
	}
	if target := strings.TrimSpace(task.Parameters["tcp"]); target != "" {
		if host, port, ok := parseEndpoint(target); ok {
			plan.TCPTargets = append(plan.TCPTargets, network.Endpoint{Host: host, Port: port})
		}
	}
	for _, rawURL := range splitTargets(task.Parameters["http"]) {
		plan.HTTPURLs = append(plan.HTTPURLs, rawURL)
	}
	for _, host := range splitTargets(task.Parameters["smb"]) {
		plan.SMBHosts = append(plan.SMBHosts, host)
	}
	checks := network.BuildChecks(plan)
	if len(checks) == 0 {
		return fmt.Errorf("no diagnostic checks configured")
	}
	report, err := network.Diagnose(ctx, checks, a.NetworkDeps)
	if err != nil {
		return err
	}
	if report.Failed > 0 {
		return fmt.Errorf("network diagnostics reported failures")
	}
	return nil
}

func gatherGateways(adapters []inventory.AdapterConfig) []string {
	result := []string{}
	for _, adapter := range adapters {
		if adapter.Operational {
			result = append(result, adapter.Gateways...)
		}
	}
	return result
}

func splitTargets(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	result := []string{}
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseEndpoint(value string) (string, int, bool) {
	index := strings.LastIndex(value, ":")
	if index <= 0 || index == len(value)-1 {
		return "", 0, false
	}
	host := strings.Trim(value[:index], "[]")
	port := 0
	for _, r := range value[index+1:] {
		if r < '0' || r > '9' {
			return "", 0, false
		}
		port = port*10 + int(r-'0')
	}
	if port <= 0 || port > 65535 {
		return "", 0, false
	}
	return host, port, true
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseBoolDefault(value string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return parseBool(value)
}

func optionalBool(parameters map[string]string, key string) (*bool, error) {
	if parameters == nil {
		return nil, nil
	}
	raw, ok := parameters[key]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value := parseBool(raw)
	return &value, nil
}

// Ensure Adapter satisfies the engine contract.
var _ engine.Adapter = (*Adapter)(nil)
