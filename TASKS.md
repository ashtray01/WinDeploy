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
  - [ ] Verify current Wails version/toolchain/Windows compatibility and create isolated Modern module
  - [ ] Add real Inventory dashboard and profile/plan preview with responsive Russian dark UI
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
adef541 — `feat(profiles): build selected plans and add workstation examples`.
Go 1.26.2: windows/amd64 — go test/vet/build ./... PASS; windows/386 — go test/build ./... PASS; go mod verify, gofmt и diff --check PASS. Go 1.20/Win7 VM/race detector не проверены.
