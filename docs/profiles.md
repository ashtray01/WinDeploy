# Профили schema v1

`core/profiles` загружает YAML, проверяет структуру, разрешает наследование и формирует Pending-план engine. GUI редактирования и системные адаптеры ещё не реализованы. Примеры в `profiles/` описывают намерения: ссылки `package_ref`, `check_ref`, `config_ref`, `share_ref` потребуют будущего каталога/адаптеров. Их загрузка не доказывает существование пакета, совместимость дистрибутива или успешную настройку.

API: `Load(io.Reader)`, `Marshal(Profile)`, `Resolve(map[string]Profile, id)`, `Clone(profile, newID, newName)`, `BuildPlan(profile, selection)`, `Preview(profile, selection)`. Вызывающий код читает и сохраняет файлы; Extends не вызывает скрытого чтения файлов или сети. Редактировать можно возвращённые структуры, затем Validate/Marshal. Clone сохраняет Extends; для автономной копии сначала Resolve.

Поля профиля: version (1), id, name, необязательный extends, components. ID: `[a-z][a-z0-9._-]{0,63}`. Категории: packages, drivers, shares, printers, browsers, diagnostics, windows. Поля компонента: id, name, category, parameters (строковые значения), requirements, dependencies, timeout (например `5m`), resource_key, disabled.

Версии ОС: windows-7-sp1, windows-10, windows-11; архитектуры: 386, amd64. Это ограничения для будущего Precheck, а не результат обнаружения ОС. Для всех операций Windows Installer задавайте resource_key: windows-installer.

Наследование — один родитель, максимум 64 уровня, без циклов. Компонент с тем же ID заменяется целиком на прежней позиции; новые добавляются в конец. Отключение унаследованного компонента: `{id: component-id, disabled: true}` без других полей. После Resolve поле extends пустое. BuildPlan отказывается от неразрешённого наследования.

Нулевой selection (`nil`) означает все включённые компоненты. Пустой ненулевой срез — ни одного. При ручном выборе зависимости включаются автоматически. Отсутствующие/отключённые зависимости, дубли выбора и циклы — ошибки. BuildPlan копирует данные, Preview исключает Parameters и показывает только проверку структуры/DAG.

YAML: один документ, до 1 MiB, неизвестные/дублирующиеся поля отклоняются. Anchors, aliases и merge keys запрещены; используйте extends. Нужен поддерживаемый YAML parser, используется go.yaml.in/yaml/v3 v3.0.4 (upstream go.mod: go 1.16). Совместимость всего проекта с Go 1.20 ещё требует проверки.

Не храните секреты. Используйте credential_ref — имя внешней записи credentials. Проверка запрещает распространённые ключи password/token/secret и URL userinfo; распознать произвольный секрет в свободном значении она не может. Ошибки парсинга не включают введённые значения. Никаких реальных серверов, ключей или дистрибутивов в примерах нет.
