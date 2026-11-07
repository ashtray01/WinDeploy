# WinDeploy Roadmap

## Current milestone
Stages 1–11 реализованы: ядро, профили, inventory, Modern GUI, Package Manager, Network & Shares, Drivers & Printers, Browsers & Windows Settings, Execution & Reporting, Legacy Windows 7 GUI, CI/CD & Release. Статус — preview: реальные изменяющие операции, Modern GUI smoke (WebView2 на хосте), Windows 7 SP1 VM и публикация релиза не проверены/недоступны. Portable/offline ZIP собраны локально.

## Tasks
- [x] Initialize Git repository
- [x] Add basic project documentation
- [x] Add Go 1.20 core skeleton (Go 1.20.14 test/vet/build amd64+386 verified)
- [x] Implement Task, TaskStatus and TaskResult models
- [x] Add first model unit tests and verify compilation
- [x] Validate plan IDs, dependency references and DAG; deterministic topological order
- [x] Implement sequential execution engine, context cancellation and task timeout
- [x] Add dry-run plan preview without Adapter calls (engine tests/vet PASS)
- [x] Add explicit retryable-error handling with bounded attempts and cancellable delay
- [x] Add bounded concurrency and exclusive resource keys
- [x] Add validated file-backed session snapshots
- [x] Resume completed tasks safely without duplicate execution
- [x] Persist checkpoints during execution and fail closed on storage errors
- [x] Stage 2 — Profiles (YAML, validation, inheritance, examples → valid plans; GUI editing later)
  - [x] Add versioned profile/component model and structural validation
  - [x] Add strict bounded YAML import/export
  - [x] Resolve single-parent inheritance with cycle detection
  - [x] Build selected-component plans, clone profiles and add examples
- [x] Stage 3 — Inventory (real current-Windows reads + parsing/error/integration tests; target VM matrix pending)
  - [x] Read Windows version/build and hostname via a read-only Windows adapter; parsing/error tests
  - [x] Read CPU model and logical processor count
  - [x] Read physical RAM totals and availability
  - [x] Enumerate local volumes and free space
  - [x] Enumerate interfaces, addresses and link state
  - [x] Read adapter gateways, DNS and DHCP configuration
  - [x] Enumerate installed apps in machine/user registry views
  - [x] Collect inventory with explicit partial errors and cancellation
  - [x] Enforce inventory-based OS/architecture applicability before task execution
  - [x] Add read-only inventory/plan-preview CLI and whole-project smoke check
- [x] Stage 4 — GUI (Modern 1–16)
  - [x] Modern 1 — Verify core with isolated Go 1.20.14: test/vet/build amd64+386 PASS (not Win7 VM proof)
  - [x] Modern 2 — Isolate and pin stable Wails v3.1.0 module; root go.mod unchanged
  - [x] Modern 3 — Build and launch Windows Wails/React/TypeScript EXE (real HWND + rendered WebView2 bootstrap smoke PASS)
  - [x] Modern 4 — Add responsive dark design system (primitives UI tests + TS/build PASS)
  - [x] Modern 5 — Add navigation shell and honest unavailable sections (UI tests/TS/build PASS)
  - [x] Modern 6 — Generated typed Wails bridge, real async Collector, cancellation/partial errors (Go/UI tests + TS/build PASS)
  - [x] Modern 7 — Real dashboard and rescan (desktop DOM matches native CLI; 1366×768 smoke PASS)
  - [x] Modern 8 — Searchable/sortable Inventory detail tables (9 UI tests + TS/build PASS)
  - [x] Modern 9 — YAML profile selection, real preview/DAG/applicability (Go tests/vet + 12 UI tests + TS/build PASS)
  - [x] Modern 10 — Real journal, errors/recovery, 14 UI tests, Windows EXE + actual GUI smoke and Go 1.20 amd64/386 regression PASS
  - [x] Modern 11 — Package catalog + detection + verified download/install engine (Go 1.20 tests amd64+386 PASS)
  - [x] Modern 12 — Package Manager GUI page with selection, progress, cancellation and elevation gating (UI tests PASS)
  - [x] Modern 13 — Headless package CLI and offline/HTTPS source verification (real 7-Zip/SumatraPDF/PrinterOne downloads PASS)
  - [x] Modern 14 — WebView2/splash startup diagnostics and bounded startup log (built; GUI smoke blocked by host WebView2 failure this session)
  - [x] Modern 15 — Network diagnostics core + GUI page (real DNS/TCP/HTTP/DNS-server checks PASS on current Windows)
  - [x] Modern 16 — UNC shares: WNet connect, Credential Manager, Error 1219 handling, desktop .lnk (native Windows tests PASS)
- [ ] Stage 5 — Package Manager
  - [x] Strict catalog schema and embedded 12-package catalog with WinGet IDs, vendor HTTPS URLs and pinned SHA-256
  - [x] Detection of installed packages (uninstall registry, MSI product code, file/portable) and version comparison
  - [x] Source selection: WinGet, HTTPS, local/USB/SMB file sources with priority, fallback and offline mode
  - [x] Streaming download with pinned SHA-256, verified content cache and Authenticode publisher checks
  - [x] Silent EXE/MSI execution, exit-code classes, retry for busy installers, RebootRequired and post-install verification
  - [x] Synchronous administrator pre-check and reinstall prevention
  - [x] Modern GUI catalog page with checkboxes, source/version/action details, live progress, cancellation and results
  - [x] Headless `windeploy-packages` CLI: list/plan/download/install (explicit --yes), JSON output
  - [ ] Install real third-party packages in an isolated Windows VM and record per-package exit codes/reboot behavior
  - [ ] Verify WinGet path in a normal interactive session (this host's sandbox and scheduled-task runs deny App Installer identity)
- [ ] Stage 6 — Network & Shares
  - [x] DNS (system + specific server), DHCP summary, gateway, TCP, ICMP, HTTP and SMB-445 diagnostics
  - [x] Bounded per-check timeouts, honest timeout/unsupported statuses and cancellation
  - [x] UNC connect via WNet with per-server Credential Manager storage (generic DPAPI credential)
  - [x] Windows Error 1219 detection and explicit conflict resolution (disconnect stale sessions and reconnect)
  - [x] Desktop .lnk creation through native IShellLink COM interface
  - [x] Modern GUI diagnostics and shares pages with confirmation, dry-run and existing-connection list
  - [ ] Connect to a real SMB server with credentials (no isolated share/test VM available)
  - [ ] Execute shares components through the profile Task Engine (planned with Stage 9 execution integration)
- [ ] Stage 7 — Drivers & Printers
  - [x] Enumerate present PnP devices, unknown/problem devices, Hardware IDs and installed driver versions (SetupAPI/CM)
  - [x] Scan local/network INF catalogs, parse Hardware IDs and DriverVer, match by exact hardware ID
  - [x] INF signature/platform verification via SetupVerifyInfFile; fail-closed on unsigned packages
  - [x] Silent INF install through pnputil with elevation, reboot codes and post-install device confirmation
  - [x] Windows Update-compatible driver rescan through pnputil /scan-devices (Windows 10/11)
  - [x] Enumerate printers, TCP/IP port queue installation from INF, shared queue name, default printer
  - [x] Real test page through GDI with explicit confirmation
  - [x] Configurable Kyocera 2460 model catalog with mandatory exact model selection
  - [x] Modern pages «Драйверы» and «Принтеры» + read-only CLI devices/printers
  - [ ] Real driver installation and test page in an isolated VM (no test device/printer available)
- [ ] Stage 8 — Browser & Windows Settings
  - [x] Chromium/Firefox enterprise policies: homepage, startup URLs, managed bookmarks
  - [x] Web application desktop shortcuts for Chrome/Edge with --app argument
  - [x] User/machine scope with dry-run, URL validation and explicit elevation
  - [x] Computer name preview/rename via SetComputerNameExW with reboot requirement
  - [x] Time zone validation and tzutil apply; Explorer options with old-value preview
  - [x] Power plan selection via powercfg with GUID validation
  - [x] Read-only Windows Update diagnostics (service, pending reboot markers, policies)
  - [x] Read-only Windows licensing status; no bypass or Massgrave
  - [x] Modern pages «Браузеры» и «Windows» with confirmations and dry-run
  - [ ] Real browser policy application and Windows setting changes in an isolated VM
- [ ] Stage 9 — Persistence & Reporting
  - [x] Deploy adapter for packages/shares/drivers/printers/browsers/windows/diagnostics components
  - [x] Resumable session store with original plan, snapshot and atomic 0600 JSON
  - [x] Start/resume/cancel integration with engine, explicit retry-failed consent
  - [x] HTML and JSON reports with escaped values, safe summaries and exclusive files
  - [x] Headless `windeploy-run` CLI (plan/run/resume/sessions/report)
  - [x] Modern «Выполнение» page with live progress, cancel, sessions and report paths
  - [x] Real read-only diagnostics deployment through CLI and service tests (session + reports)
  - [ ] Execute modifying components end-to-end in an isolated VM, including reboot continuation
- [ ] Stage 10 — Legacy Windows 7
  - [x] Separate Go 1.20 module with native Walk Win32 GUI, no WebView2/.NET
  - [x] System summary, compatible package list, install/cancel, network diagnostics and UNC shares
  - [x] Common Controls 6 manifest embedded for x64/x86 (fixes Walk TTM_ADDTOOL)
  - [x] Legacy unit tests, `--selftest`, real GUI `--smoke` x64/x86 on Windows 11
  - [x] `scripts/build-legacy.ps1` builds both architectures and hashes outputs
  - [ ] Windows 7 SP1 VM runtime and real install/SMB tests
- [ ] Stage 11 — CI/CD & Release
  - [x] GitHub Actions CI for core/legacy/modern + full Wails EXE build
  - [x] Release workflow for tag/manual with pre-release publishing
  - [x] Portable x64/x86 and offline bundle packaging with SHA256SUMS
  - [x] Verified offline artifacts: 7-Zip, SumatraPDF, PrinterOne
  - [ ] CI executed on GitHub-hosted runners (no remote in this workspace)
  - [ ] Publish v0.1.0 pre-release (requires remote + unmet VM/GUI-smoke criteria)

## Blocked
GitHub CLI отсутствует; remote не настроен; CI workflow не запускался. Локальная разработка и preview-артефакты доступны.
Windows 7 SP1 VM отсутствует; Legacy проверен только на Windows 11 (реальное окно x64/x86), что не доказывает Win7 runtime.
WebView2 в текущей сессии хоста не создаёт контроллер (0x8000FFFF) даже для минимального Wails-приложения; Modern GUI smoke заблокирован внешней средой, не кодом WinDeploy.
Установка программ/драйверов/принтеров, SMB и политики/настройки требуют изолированной VM/тестового сервера; на рабочем хосте не выполнялись. Запись в HKCU\Software блокируется низким integrity тестового процесса.

## Last verified session
Финальная проверка Stages 1–11: root Go 1.20.14 test/vet/build amd64+386 PASS; Modern Wails build, 37 UI-тестов, strict TypeScript, Vite, service tests PASS; Legacy test/vet x64/x86, selftest и реальный GUI smoke (окно + auto-close) PASS на Windows 11; read-only deployment создал сессию и HTML/JSON отчёты; 5 CLI-тестов `windeploy-run` PASS; workflows прошли YAML parse и actionlint. Portable x64/x86 и offline x64 ZIP + SHA256SUMS собраны. Реальные установки, Win7/10 VM, Modern WebView2 smoke и публикация не выполнялись. Детали: HANDOFF.md, docs/release.md, docs/legacy.md, docs/execution.md.

## Audit iteration — 2026-11-07
- [x] P0: reject installation Success when post-install detection fails; regression reproduced before fix, packages/deploy/CLI tests and vet PASS.

