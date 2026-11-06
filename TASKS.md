# WinDeploy Roadmap

## Current milestone
Stage 4 — GUI: Inventory и read-only CLI проверены на доступной Windows (amd64 и 386). Следующая задача — проверка совместимости Wails и отдельный Modern-модуль; Go 1.20/Win7 VM остаются открытыми.

## Tasks
- [x] Initialize Git repository
- [x] Add basic project documentation
- [x] Add Go 1.20 core skeleton (директива модуля; Go 1.20 toolchain ещё не проверен)
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
  - [ ] Modern 1 — Verify core with isolated Go 1.20 on amd64/386 (official archive transport interrupted; resume pending; root Go 1.26 regressions PASS)
  - [x] Modern 2 — Isolate and pin stable Wails v3.1.0 module; root go.mod unchanged
  - [x] Modern 3 — Build and launch Windows Wails/React/TypeScript EXE (real HWND + rendered WebView2 bootstrap smoke PASS)
  - [ ] Modern 4 — Add responsive dark design system
  - [ ] Modern 5 — Add navigation shell and honest unavailable sections
  - [ ] Modern 6 — Typed Go bridge with real Inventory, cancellation and partial errors
  - [ ] Modern 7 — Real dashboard and rescan
  - [ ] Modern 8 — Searchable/sortable Inventory detail tables
  - [ ] Modern 9 — YAML profile selection, real preview/DAG/applicability
  - [ ] Modern 10 — Real journal, UI/backend tests, GUI smoke and core regression check
- [ ] Stage 5 — Package Manager
- [ ] Stage 6 — Network & Shares
- [ ] Stage 7 — Drivers & Printers
- [ ] Stage 8 — Browser & Windows Settings
- [ ] Stage 9 — Persistence & Reporting
- [ ] Stage 10 — Legacy Windows 7
- [ ] Stage 11 — CI/CD & Release

## Blocked
GitHub CLI отсутствует; remote не настроен. Локальная разработка доступна.
Проверка Go 1.20 и Windows 7 VM не выполнена; совместимость не подтверждена.

## Last verified commit
1ff69d4 — `feat(cli): add read-only inventory and profile preview commands`.
Go 1.26.2: windows/amd64 — go test/vet/build ./... PASS; windows/386 — go test/build ./... PASS; linux/amd64 cross-build PASS. Real x64/x86 CLI inventory + 8 profile previews PASS; go mod verify/gofmt/diff --check PASS. Go 1.20/Win7 VM/race detector не проверены. Подробности: docs/reports/autonomous-inventory.md.
