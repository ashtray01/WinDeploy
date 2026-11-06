# WinDeploy Roadmap

## Current milestone
Stage 4 — первый Modern GUI с настоящими Inventory и Profiles работает на текущей Windows 11. Все 10 задач пакета выполнены; дальнейший GUI hardening и VM-матрица остаются открытыми. Package Manager не начат.

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
- [ ] Stage 4 — GUI
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
- [ ] Stage 5 — Package Manager
- [ ] Stage 6 — Network & Shares
- [ ] Stage 7 — Drivers & Printers
- [ ] Stage 8 — Browser & Windows Settings
- [ ] Stage 9 — Persistence & Reporting
- [ ] Stage 10 — Legacy Windows 7
- [ ] Stage 11 — CI/CD & Release

## Blocked
GitHub CLI отсутствует; remote не настроен. Локальная разработка доступна.
Go 1.20.14 проверен на текущей Windows; Windows 7 SP1 VM и полная целевая VM-матрица пока не проверены.

## Last verified session
Modern 1–10: root Go 1.20.14 test/vet/build amd64+386 PASS; Modern Go tests/vet/build + module cache verify PASS; 14 UI tests/typecheck/Vite PASS. Реальный Windows desktop smoke: native Inventory comparison, rescan, tables, Profiles/DAG/Legacy incompatibility, journal, 1366×768 и 1024×640 PASS; 0 page errors. Подробности: docs/modern.md, docs/reports/modern-gui.md и HANDOFF.md.
