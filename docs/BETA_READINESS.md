# WinDeploy — Beta Readiness

Дата среза: 2026-11-08. Целевая версия: **v0.1.0-beta.1**. Это не повторный
аудит и не заявление о production readiness. Матрица описывает только то, что
подтверждено кодом, тестами и реальными проверками на текущем хосте.

## Итог

**Статус: beta candidate, публиковать пока нельзя.** Инфраструктура Modern-only
сборки и release gate подготовлены, но остаются блокеры B1, B2, B3, B6 и
требуется явное решение B8.

| # | Блокер | Доказательство | Что нужно |
|---|---|---|---|
| B1 | Основной сценарий установки не подтверждён реальной установкой | Есть unit/service/UI-тесты и собранный EXE; в этой итерации не запускался ни один установщик. Нет clean-install acceptance и проверки обновления/отмены | На Win11 x64 установить один согласованный безопасный пакет (например, 7-Zip) с разрешения владельца; detection до/после, повтор, update, failure/cancel, отчёт |
| B2 | SMB-подключение и обязательный ярлык не проверены с реальным сервером | Native list и временный `.lnk` read-back PASS; сервер/credentials/конфликт 1219 не проверялись | Тестовый SMB-сервер и отдельная учётная запись; подключение, ярлык, повтор, отказ, отсутствие секретов в отчёте |
| B3 | Лицензия не выбрана, публичный URL/владелец не зафиксированы | `LICENSE` — осознанная заглушка; remote отсутствует | Владелец выбирает лицензию и публичный репозиторий; не открывать исходники до этого |
| B4 | ~~Git-история не проверена на секреты и персональные данные~~ | **Закрыт для текущего среза:** `check-release-secrets.ps1 -IncludeHistory` — 0 blocking findings; manual-review items — тестовые/документационные примеры. Повторить после финального docs commit | Владелец проверяет findings; повторный прогон перед открытием repo |
| B5 | ~~Нет подтверждённого desktop smoke этой beta-сборки~~ | **Закрыт 2026-11-08:** actual GUI EXE DOM/bridge/native Inventory smoke PASS; native hostname/app count совпали, 2 viewport, pageErrors=0; screenshots обезличены | Повторять при изменении UI/EXE |
| B6 | Нет GitHub-hosted run и draft Release | Remote отсутствует; workflow подготовлены | Создать remote только с разрешения владельца, выполнить CI, затем draft release |
| B7 | Windows 10/LTSC не проверены | Нет VM; текущий хост — Windows 11 | **Решено:** beta scope — только Windows 11 x64; Win10/LTSC не заявляются |
| B8 | Нет явного подтверждения владельца на публикацию | Пользователь разрешил подготовку, но не публичный Release/репозиторий | Владелец проверяет draft/документы и отдельно разрешает открытие repo/Release |

B3 и B6 — release-процессные блокеры. B1–B2 — функциональные. B4 закрыт для
текущего среза, B5 закрыт фактическим smoke, B7 закрыт сужением supported scope.
B8 нельзя закрыть технической проверкой.

## Матрица возможностей

Обозначения: **Да** — проверено на текущем Windows 11; **Тесты** — только
unit/integration/UI без реального системного эффекта; **Нет** — не проверялось.

| Функция | Реальное состояние | Доступна в GUI | Проверена на Windows 11 | Известные проблемы | Можно включить в beta | Требуется отключить | Исправить до выпуска |
|---|---|---|---|---|---|---|---|
| Inventory: система, CPU, RAM, тома, сеть, программы | Native collector; partial errors отображаются | Да | Да (actual GUI smoke 2026-11-08; native count совпал) | Win10/LTSC, AppX/MSIX/portable не входят | Да | Нет | Нет |
| Каталог программ и detection | 12 закреплённых пакетов, WinGet/file/HTTPS, SHA-256/подпись | Да | Тесты + read-only каталог | Версии каталога стареют; target-version update не подтверждается отдельно | Да | Нет | Проверить на одном пакете (B1) |
| Выбор программ | Явный выбор, подтверждение, UAC-баннер | Да | Тесты; auto-select удалён | Нет реального installer run | Да | Нет | B1 |
| Установка/обновление пакетов | Pipeline: elevation, download, trust, install, detect | Да | Тесты; реальной установки нет | Target version/update, child processes, cancel — не VM-accepted | Да, как **beta** | Нет | B1 |
| Прогресс/отмена/результат установки | Polling, события, cooperative cancel, честный failed; пустой экран не выдаётся за завершённый | Да | UI 6 tests; backend tests | Polling fail показывает stale; child installer tree не гарантирован | Да | Нет | B1 |
| Профили и DAG | Strict YAML, inheritance, BuildPlan, read-only preview | Да | Тесты + предыдущий DAG smoke | Semantic parameters/check_ref не проверяются полностью | Да | Нет | Нет |
| Основной маршрут выполнения | Preview → confirm → progress → result → report; Legacy не выбирается по умолчанию | Да | UI 7 tests + actual GUI smoke | Реальный install внутри профиля не проверен | Да | Нет | B1 |
| Network diagnostics | DNS/TCP/ICMP/HTTP/SMB read-only | Да | Предыдущий native/GUI smoke | Нет ручной отмены; Win10/LTSC нет | Да | Нет | Нет |
| SMB-подключение и ярлыки | WNet/CredMan/IShellLink, required shortcut read-back | Да | Тесты + native `.lnk`; реального сервера нет | 1219 требует подтверждения; credential context не VM-accepted | Условно | Нет | B2 |
| Отчёт HTML/JSON | Escaped HTML, JSON, warnings, без parameters | Да | File tests | Точность зависит от операций; ACL отчётов не отдельная граница | Да | Нет | Нет |
| Session resume/reboot | Bounded parse, trust/owner, boot evidence, verify-only | Да | Тесты | Реальный reboot и fast startup не проверены; ACL ≠ authentication | Да, с ограничением | Нет | Нет (документировать) |
| User/SID gate | Process SID = shell owner, fail closed | Да (backend) | Native SID read + negative tests | Two-account/RDP/headless VM не проверены | Да | Нет | Нет |
| Драйверы | Перечисление и подписанный INF install | Да | Read-only тесты; install не проверялся | Target version/publisher, VM, rollback | **Эксперимент** | Нет (явное предупреждение) | Нет |
| Принтеры | Enumeration, default/read-back, INF install | Да | Read-only; install нет | TCP/IP port не создаётся (P1-01), печать/VM нет | **Эксперимент** | Добавление принтера не обещать | Нет |
| Политики браузеров | Registry write + read-back, webapps | Да | Fake-store + negative tests; VM нет | Полная VM-проверка и восстановление старых значений не закрыты | **Эксперимент** | Нет | Нет |
| Windows settings | Rename/timezone/Explorer/power, preview/read-back | Да | Read-only API + fake-store; VM нет | Rename/reboot/домен, restore, VM | **Эксперимент** | Нет | Нет |
| Modern CI/release | Modern-only CI, tag/version check, SHA256, draft release | Нет (CI) | Сборка/тесты локально | GitHub run, remote, signing | Инфраструктура готова | Нет | B4–B6 |
| Windows 7 Legacy | Отдельный frozen-модуль | Нет в beta | Не проверялась | FROZEN | Нет | Да, не включать | Нет |

## Что включается в beta

- Windows 11 x64, Modern GUI.
- Inventory, профили, каталог программ, явная установка выбранного ПО,
  подтверждение, прогресс, отмена, результат и HTML/JSON-отчёт.
- Network diagnostics read-only.
- SMB-подключение и ярлыки — только как beta-функция после B2.
- Session resume без false success; реальный reboot acceptance остаётся
  ограничением и не заявляется как проверенный.

## Что не включается / явно экспериментально

- Windows 7 Legacy — FROZEN, в артефакт не входит.
- Windows 10/LTSC — не заявляются проверенными.
- Драйверы, добавление принтеров, browser policies, Windows settings — доступны
  только с предупреждением «экспериментально, без VM-приёмки».
- Code signing, auto-update, offline bundle как release gate, full reboot
  continuation — POST-BETA.

## Security gates, закрытые для beta

- Session files: bounded read, strict JSON, ID binding, owner/ACL trust перед
  изменяющим resume; negative tests.
- SID-контекст: user-scoped операции отклоняются при unknown/mismatch.
- Установщики: pinned SHA-256 и/или Authenticode, fail closed; cache artifact
  проверяется по trusted ACL.
- Warnings о неподтверждённых эффектах сохраняются в result/session/report/UI.
- Отсутствие обязательного ярлыка не даёт `Success`.
- Пароли не попадают в task parameters/отчёты; отчёты не содержат parameters.

## Остаточные риски

- Owner/ACL проверка сессии не аутентифицирует автора и не защищает от
  same-user/administrator tampering. Elevated resume автоматически не
  используется; при другом SID операция отклоняется.
- Отсутствует отдельный elevated worker; SID gate не замена two-account VM.
- Real install/SMB/reboot acceptance остаётся обязательным до публикации.
- EXE не подписан; это должно быть в release notes и Known Issues.
- License/public repository URL/owner confirmation — обязательные действия
  владельца, не технические задачи агента.

## Проверено в этой итерации

- **B5 закрыт:** actual GUI EXE DOM/bridge/native Inventory smoke PASS
  (`actualDesktop`, hostname/app count matched, 2 viewport, pageErrors=0).
- Screenshots `docs/images/` сняты с этой сборки; hostname заменён на
  `DEMO-WORKSTATION`.

## Следующее обновление статуса

После B1/B2 и любых новых проверок этот файл обновляется. Не отмечать B1/B2/B6
закрытыми без воспроизводимого лога, реального сервера/пакета и подтверждения
GitHub. B5 закрыт только для текущей сборки; UI-изменения требуют нового smoke.
