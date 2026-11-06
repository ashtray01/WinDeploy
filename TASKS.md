# WinDeploy Roadmap

## Current milestone
Stage 2 — Profiles: Core Foundation расширен dry-run, retries, concurrency и checkpoints/resume. Реальная проверка ОС и Go 1.20 остаётся открытой.

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
- [ ] Stage 2 — Profiles
  - [x] Add versioned profile/component model and structural validation
  - [x] Add strict bounded YAML import/export
  - [x] Resolve single-parent inheritance with cycle detection
  - [ ] Build selected-component plans, clone profiles and add examples
- [ ] Stage 3 — Inventory
- [ ] Stage 4 — GUI
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
36fb704 — `feat(engine): add sequential task executor`; go test ./core/engine/..., go vet ./..., go build ./... и gofmt прошли на Go 1.26.2 windows/amd64.
