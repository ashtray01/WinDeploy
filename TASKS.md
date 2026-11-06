# WinDeploy Roadmap

## Current milestone
Stage 1 — Core Foundation: последовательный исполнитель завершён; следующая часть — dry-run.

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
- [ ] Add bounded concurrency and exclusive resource keys
- [ ] Add validated file-backed session snapshots
- [ ] Resume completed tasks safely without duplicate execution
- [ ] Persist checkpoints during execution and fail closed on storage errors
- [ ] Stage 2 — Profiles
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
