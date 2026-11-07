import * as Service from '../bindings/windeploy/modern/service/service';
export type { State, Entry } from '../bindings/windeploy/modern/service/models';
export type { Catalog, ProfileDetails, ProfilePreview, PlannedTask } from '../bindings/windeploy/modern/service/models';
export type { PackageCatalogView, PackageView, PackageInstallView, PackageResultView, PackageEvent, PackageEnvironment } from '../bindings/windeploy/modern/service/models';
export type { NetworkReportView, NetworkCheckView } from '../bindings/windeploy/modern/service/models';
export type { ShareRequest, ShareResultView, ShareConnectionView } from '../bindings/windeploy/modern/service/models';
export type { DriverInstallRequest } from '../bindings/windeploy/modern/service/models';
export type { BrowserEnvironmentView, PowerPlanOption, WindowsSettingsView } from '../bindings/windeploy/modern/service/models';
export type { DeploymentView, DeployTaskView, DeployEvent, DeploySessionView, AppInfo } from '../bindings/windeploy/modern/service/models';
export type { Device as DriverDevice, INFInfo as DriverInfo, Result as DriverResult } from '../bindings/windeploy/core/drivers/models';
export type { Printer as PrinterInfo, AddRequest as PrinterAddRequest, Result as PrinterResult } from '../bindings/windeploy/core/printers/models';
export type { Policy as BrowserPolicy, Bookmark as BrowserBookmark, WebApp as BrowserWebApp, Result as BrowserResult } from '../bindings/windeploy/core/browsers/models';
export { Browser as BrowserKind, Scope as BrowserScope } from '../bindings/windeploy/core/browsers/models';
export type { Result as SettingsResult, ExplorerOptions, UpdateReport, Activation } from '../bindings/windeploy/core/settings/models';
export type { Report, SystemInfo, CPUInfo, Volume, InterfaceInfo, InstalledApp, AdapterConfig } from '../bindings/windeploy/core/inventory/models';

// This is the only production backend transport. No HTTP fallback or sample data.
export const api = {
  appInfo: Service.GetAppInfo,
  state: Service.GetState,
  startScan: Service.StartScan,
  cancelScan: Service.CancelScan,
  journal: Service.GetJournal,
  listProfiles: Service.ListProfiles,
  getProfile: Service.GetProfile,
  previewProfile: Service.PreviewProfile,
  getPackages: Service.GetPackages,
  startPackageInstall: Service.StartPackageInstall,
  getPackageInstall: Service.GetPackageInstall,
  cancelPackageInstall: Service.CancelPackageInstall,
  runNetworkDiagnostics: Service.RunNetworkDiagnostics,
  addShare: Service.AddShare,
  listShareConnections: Service.ListShareConnections,
  getDevices: Service.GetDevices,
  scanDriverCatalog: Service.ScanDriverCatalog,
  installDriver: Service.InstallDriver,
  scanDevicesForUpdates: Service.ScanDevicesForUpdates,
  getPrinters: Service.GetPrinters,
  addPrinter: Service.AddPrinter,
  setDefaultPrinter: Service.SetDefaultPrinter,
  printTestPage: Service.PrintTestPage,
  getKyocera2460Models: Service.GetKyocera2460Models,
  getBrowserEnvironment: Service.GetBrowserEnvironment,
  applyBrowserPolicy: Service.ApplyBrowserPolicy,
  getWindowsSettings: Service.GetWindowsSettings,
  applyComputerName: Service.ApplyComputerName,
  applyTimezone: Service.ApplyTimezone,
  applyExplorer: Service.ApplyExplorer,
  applyPowerPlan: Service.ApplyPowerPlan,
  getWindowsUpdateDiagnostics: Service.GetWindowsUpdateDiagnostics,
  getPowerPlanOptions: Service.GetPowerPlanOptions,
  startDeployment: Service.StartDeployment,
  getDeployment: Service.GetDeployment,
  cancelDeployment: Service.CancelDeployment,
  resumeDeployment: Service.ResumeDeployment,
  listDeploymentSessions: Service.ListDeploymentSessions,
  getDeploymentSession: Service.GetDeploymentSession,
};
