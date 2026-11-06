# WinDeploy Roadmap

## Current milestone
Stage 1 — Core Foundation: модели завершены; следующая часть — валидация DAG.

## Tasks
- [x] Initialize Git repository
- [x] Add basic project documentation
- [x] Add Go 1.20 core skeleton (директива модуля; Go 1.20 toolchain ещё не проверен)
- [x] Implement Task, TaskStatus and TaskResult models
- [x] Add first model unit tests and verify compilation
- [ ] Validate plan IDs, dependency references and DAG; deterministic topological order (следующая сессия)
- [ ] Implement execution engine and context cancellation
- [ ] Implement retries, dry-run and concurrency control
- [ ] Implement persistent state and resume
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
7dd54cf — bootstrap. Модели проверены в рабочем дереве; хеш их коммита будет записан после фиксации.
