import * as Service from '../bindings/windeploy/modern/service/service';
export type { State, Entry } from '../bindings/windeploy/modern/service/models';
export type { Catalog, ProfileDetails, ProfilePreview, PlannedTask } from '../bindings/windeploy/modern/service/models';
export type { PackageCatalogView, PackageView, PackageInstallView, PackageResultView, PackageEvent, PackageEnvironment } from '../bindings/windeploy/modern/service/models';
export type { Report, SystemInfo, CPUInfo, Volume, InterfaceInfo, InstalledApp, AdapterConfig } from '../bindings/windeploy/core/inventory/models';

// This is the only production backend transport. No HTTP fallback or sample data.
export const api = {
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
};
