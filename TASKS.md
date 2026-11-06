# WinDeploy Roadmap

## Current milestone
Stage 5 «Package Manager» и Stage 6 «Network & Shares» реализованы в ядре и Modern GUI: каталог из 12 программ, реальная детекция/план/загрузка с SHA-256, установка через WinGet/EXE/MSI с проверкой подписи, страница установки с прогрессом, диагностика сети и подключение UNC с Windows Credential Manager. Следующий этап — драйверы и принтеры. Реальная установка программ и SMB-подключение в изолированной VM ещё не выполнялись.

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
- [ ] Stage 8 — Browser & Windows Settings
- [ ] Stage 9 — Persistence & Reporting
- [ ] Stage 10 — Legacy Windows 7
- [ ] Stage 11 — CI/CD & Release

## Blocked
GitHub CLI отсутствует; remote не настроен. Локальная разработка доступна.
Go 1.20.14 проверен на текущей Windows; Windows 7 SP1 VM и полная целевая VM-матрица пока не проверены.
WebView2 в текущей сессии хоста не создаёт контроллер (0x8000FFFF) даже для минимального Wails-приложения; реальный GUI smoke этого пакета заблокирован внешней средой, не кодом WinDeploy.
Установка программ и подключение SMB требуют изолированной VM/тестового сервера; на рабочем хосте не выполнялись.

## Last verified session
Package Manager: root Go 1.20.14 test/vet/build amd64+386 PASS; Modern Go tests/vet/build PASS; 23 UI tests/typecheck/Vite PASS. Real host: detection of installed apps, HTTPS downloads with pinned SHA-256 and cache reuse for 7-Zip, SumatraPDF and PrinterOne; network diagnostics (DNS, specific DNS server, TCP 445, HTTP, DHCP, gateway) executed on current Windows; Credential Manager round-trip, WNet enumeration and .lnk creation PASS in native Windows tests. Details: docs/packages.md, docs/network.md, HANDOFF.md.
