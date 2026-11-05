# Live Code Mutation Engine — план реализации

> **Для Claude Code и Codex:** общие правила работы — в `AGENTS.md`. Перед началом задачи обновляйте её статус ниже, затем ведите чекбоксы шагов. Если доступны навыки `superpowers:subagent-driven-development` или `superpowers:executing-plans`, используйте подходящий; их отсутствие не блокирует работу по общим правилам.

## Статус выполнения

`T0`–`T18` соответствуют существующим разделам `Task 0`–`Task 18`; нумерацию не менять. Служебные изменения вне этих задач получают отдельный ID, например `DOCS-1`.

Таблица хранит текущее состояние затронутых задач, журнал — историю переходов. Обновляет их ведущий агент. Перед работой добавить или обновить строку и записать «начата работа над T<N>»; после реализации — «T<N> требует проверки»; после успешных проверок — «T<N> завершена». Для блокировки указать причину и следующий шаг. Время — `Asia/Almaty` (UTC+5).

Статусы: `не начата`, `в работе`, `требует проверки`, `заблокирована`, `завершена`, `не сверено`. Чекбокс `[x]` означает проверенный выполненный шаг, а не всю задачу. Нет строки в таблице — состояние ещё не сверено, а не «ничего не сделано»: в репозитории уже есть реализация части плана. Не восстанавливать готовность только по наличию файлов или старому сообщению агента.

| ID | Задача | Статус | Исполнитель / ветка | Обновлено (UTC+5) | Проверка / следующий шаг |
|---|---|---|---|---|---|
| DOCS-1 | Общие инструкции Claude Code и Codex, субагенты и статусы плана | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 13:44 | Context7 и официальные источники сверены; импорт, сохранность исходного плана и diff проверены; независимое ревью без замечаний |
| AUDIT-1 | Проверка текущего состояния проекта | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:12 | Снимок аудита на 14:12: make test, textkit, go vet и diff прошли; ошибка Store воспроизведена; MVP не собирается без T8–T12, Docker не проверен; далее исправления и T8 |
| T0 | Окружение | не сверено | Codex (сверка) / `codex/shared-agent-instructions` | 2026-11-05 14:07 | На этом Mac Go 1.27.3 найден в ~/.local/go/bin, Node 22.20.0; Docker/ngrok не найдены; целевое WSL-окружение не проверялось |
| T1 | Каркас и textkit | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:43 | make build с чистым npm ci прошёл; typecheck, 10 UI-тестов, независимое ревью и браузерный textkit E2E прошли. Исправление 2→3 зелёных, отметка/принятие, сохранение дедлайна после reload и отчёт проверены; pageerror отсутствуют |
| T2 | Мутации | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:33 | Исправления проверены независимым ревью без замечаний; make test и go vet прошли, audit дополнительно прошёл -race. Регрессии: //line, ограничение пакетов кандидатов, владение результатами и откат, граница дедлайна и сохранение запроса завершения |
| T3 | Протокол, раннер и Local | требует проверки | Codex (сверка) / `codex/shared-agent-instructions` | 2026-11-05 14:07 | Локальные тесты прошли; до публичного запуска проверить достоверность pass: сейчас основан на коде выхода процесса, отдельного подтверждения выполнения теста нет |
| T4 | Отбор тройки | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:33 | Исправления проверены независимым ревью без замечаний; make test и go vet прошли, audit дополнительно прошёл -race. Регрессии: //line, ограничение пакетов кандидатов, владение результатами и откат, граница дедлайна и сохранение запроса завершения |
| T5 | Docker-песочница | требует проверки | Codex (сверка) / `codex/shared-agent-instructions` | 2026-11-05 14:07 | Реализация и fake-Docker тесты есть; make test-docker, сборка образа, изоляция и замер <5 с не проверены: Docker недоступен |
| T6 | Импорт | требует проверки | Codex (сверка) / `codex/shared-agent-instructions` | 2026-11-05 14:07 | Тесты прошли; перед публикацией ограничить clone по ресурсам и доступ к внутренним HTTPS-адресам; текущие лимиты дерева действуют после clone |
| T7 | Сессия, отчёт, хранилище | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:33 | Исправления проверены независимым ревью без замечаний; make test и go vet прошли, audit дополнительно прошёл -race. Регрессии: //line, ограничение пакетов кандидатов, владение результатами и откат, граница дедлайна и сохранение запроса завершения |
| T8 | HTTP, WebSocket и сервер | требует проверки | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:43 | Локальная реализация и проверки завершены: make test, go vet, race API/audit/hints/config, make build, браузерный E2E textkit и независимое ревью прошли. Не выполнены Docker и публичный сценарий; T13 реальный импорт/мобильная сеть и T15 публичная проверка остаются открыты |
| T9 | Каркас фронтенда | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:43 | make build с чистым npm ci прошёл; typecheck, 10 UI-тестов, независимое ревью и браузерный textkit E2E прошли. Исправление 2→3 зелёных, отметка/принятие, сохранение дедлайна после reload и отчёт проверены; pageerror отсутствуют |
| T10 | Экран студента | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:43 | make build с чистым npm ci прошёл; typecheck, 10 UI-тестов, независимое ревью и браузерный textkit E2E прошли. Исправление 2→3 зелёных, отметка/принятие, сохранение дедлайна после reload и отчёт проверены; pageerror отсутствуют |
| T11 | Экран аудитора | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:43 | make build с чистым npm ci прошёл; typecheck, 10 UI-тестов, независимое ревью и браузерный textkit E2E прошли. Исправление 2→3 зелёных, отметка/принятие, сохранение дедлайна после reload и отчёт проверены; pageerror отсутствуют |
| T12 | Отчёт UI и сборка | завершена | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:43 | make build с чистым npm ci прошёл; typecheck, 10 UI-тестов, независимое ревью и браузерный textkit E2E прошли. Исправление 2→3 зелёных, отметка/принятие, сохранение дедлайна после reload и отчёт проверены; pageerror отсутствуют |
| T13 | Публичный запуск и сквозной прогон | требует проверки | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:43 | Локальная реализация и проверки завершены: make test, go vet, race API/audit/hints/config, make build, браузерный E2E textkit и независимое ревью прошли. Не выполнены Docker и публичный сценарий; T13 реальный импорт/мобильная сеть и T15 публичная проверка остаются открыты |
| T14 | Подсказки аудитору | требует проверки | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:33 | Реализация и локальные HTTP-тесты прошли независимое ревью, make test и -race; интегрировано с API. Ручная проверка с настоящим ключом OpenRouter не выполнялась |
| T15 | Сигналы и QR | требует проверки | Codex / `codex/shared-agent-instructions` | 2026-11-05 14:43 | Локальная реализация и проверки завершены: make test, go vet, race API/audit/hints/config, make build, браузерный E2E textkit и независимое ревью прошли. Не выполнены Docker и публичный сценарий; T13 реальный импорт/мобильная сеть и T15 публичная проверка остаются открыты |
| T16 | Живая проверка | не сверено | Codex (сверка) / `codex/shared-agent-instructions` | 2026-11-05 14:07 | Локальное демо собрано и проверено автоматически; живой прогон человеком не проводился |
| T17 | Презентация и видео | не сверено | Codex (сверка) / `codex/shared-agent-instructions` | 2026-11-05 14:07 | Артефактов в checkout нет; внешние материалы и репетиции не проверялись |
| T18 | Форма сдачи | не сверено | Codex (сверка) / `codex/shared-agent-instructions` | 2026-11-05 14:07 | Внешняя форма и факт сдачи не проверялись |
| REL-1 | Публикация локального этапа в main | в работе | Codex / `main` | 2026-11-05 14:47 | origin/main — предок проверенного bc82b77; подготовить fast-forward и обычный push, затем сверить удалённый SHA |

### Журнал изменений статуса

- 2026-11-05 14:47 (UTC+5) — начата работа над REL-1: пользователь поручил push в main; fetch выполнен, origin/main является предком HEAD bc82b77, расхождений нет. Код уже прошёл проверки предыдущего этапа и с тех пор не менялся; выполняется fast-forward без force push.
- 2026-11-05 14:45 (UTC+5) — локальный этап сохранён коммитом `0aa14bd` (53 файла); push/merge не выполнялись. Все статусы выше относятся к проверенному состоянию этого коммита. Локальное демо оставлено на `127.0.0.1:18080` с временными данными для просмотра; штатный повторный запуск — `make demo`. Внешние/Docker критерии и замечания T3/T6 остаются открыты.
- 2026-11-05 14:45 (UTC+5) — передача non-Docker этапа: изменены ядро `internal/mutate`, `selector`, `audit`; добавлены API/WS, `cmd/server`, hints/OpenRouter, весь React/Vite/Monaco интерфейс `web`, scripts и цели Makefile; обновлены README/.env.example/зависимости. Проверки: `make test`, textkit, `go vet ./...`, `go test -race ./internal/api ./internal/audit ./internal/hints ./cmd/server`, `make build` с чистым `npm ci`, typecheck, 10 UI-тестов, shell syntax, diff-check — PASS. Независимые ревью ядра, API и UI пройдены. Реальный браузер Chromium: подготовка textkit, старт, 2→3 зелёных после одной правки с немедленным Run, отметка/принятие, reload без смены deadline, финальный отчёт и отсутствие pageerror — PASS. Исправлены найденные ревью потеря ACK-версии при обрыве и источник подсказки, а также найденное браузером освобождение моделей DiffEditor. Проверка использовала временные data/work; пользовательские репозитории не запускались. Дальше: замечания T3/T6 до публичного доступа, Docker T5/T8/T13, реальный ключ T14, публичный сценарий T15 и пользовательский T16. Внешние публикации и отправка формы не выполнялись. Vite предупреждает о размере отложенного Monaco-модуля (~844 КиБ gzip); сборка успешна. Подготовлен локальный коммит, без push/merge.
- 2026-11-05 14:43 (UTC+5) — T8,T13,T15 требует проверки: Локальная реализация и проверки завершены: make test, go vet, race API/audit/hints/config, make build, браузерный E2E textkit и независимое ревью прошли. Не выполнены Docker и публичный сценарий; T13 реальный импорт/мобильная сеть и T15 публичная проверка остаются открыты. Исполнитель — Codex, ветка `codex/shared-agent-instructions`.
- 2026-11-05 14:43 (UTC+5) — T1,T9,T10,T11,T12 завершена: make build с чистым npm ci прошёл; typecheck, 10 UI-тестов, независимое ревью и браузерный textkit E2E прошли. Исправление 2→3 зелёных, отметка/принятие, сохранение дедлайна после reload и отчёт проверены; pageerror отсутствуют. Исполнитель — Codex, ветка `codex/shared-agent-instructions`.
- 2026-11-05 14:35 (UTC+5) — T9,T10,T11,T12,T15 требует проверки: Интерфейс готов, typecheck/build и 8 frontend-тестов прошли у исполнителя; проверяется штатный npm ci, локальный браузерный сценарий и независимое ревью. Исполнитель — Codex, ветка `codex/shared-agent-instructions`.
- 2026-11-05 14:33 (UTC+5) — T14 требует проверки: Реализация и локальные HTTP-тесты прошли независимое ревью, make test и -race; интегрировано с API. Ручная проверка с настоящим ключом OpenRouter не выполнялась. Исполнитель — Codex, ветка `codex/shared-agent-instructions`.
- 2026-11-05 14:33 (UTC+5) — T2,T4,T7 завершена: Исправления проверены независимым ревью без замечаний; make test и go vet прошли, audit дополнительно прошёл -race. Регрессии: //line, ограничение пакетов кандидатов, владение результатами и откат, граница дедлайна и сохранение запроса завершения. Исполнитель — Codex, ветка `codex/shared-agent-instructions`.
- 2026-11-05 14:30 (UTC+5) — T8 требует проверки: API, WebSocket, фоновые прогоны и восстановление завершения реализованы; целевые race-тесты прошли, идут независимое ревью и интеграция с интерфейсом. Docker-критерий остаётся открытым. Исполнитель — Codex, ветка `codex/shared-agent-instructions`.
- 2026-11-05 14:27 (UTC+5) — T2,T4,T7 требует проверки: Исправления ядра готовы: физические строки, глубокая копия результатов, точный дедлайн, заморозка завершения; регрессионные проверки прошли, полный прогон пакетов и независимое ревью ещё выполняются. Решение T4: вместо единого Batch кандидатов максимум 16 вариантов и 32 МиБ за пакет с накопительным прогрессом. Исполнитель — Codex, ветка `codex/shared-agent-instructions`.
- 2026-11-05 14:26 (UTC+5) — T13 в работе: Добавлены scripts/run.sh, scripts/dev.sh и make demo для доверенной локальной проверки; публичный запуск и Docker остаются за пределами доступной проверки. Исполнитель — Codex, ветка `codex/shared-agent-instructions`.
- 2026-11-05 14:25 (UTC+5) — T14 требует проверки: добавлены `internal/hints/hints_test.go`, `openrouter.go`, расширен `hints.go`. Тесты шаблонов всех видов, запроса, некорректного/неполного ответа, таймаута и ±3 строк прошли (`go test -timeout 30s ./internal/hints`). Формат API сверен Context7; реальные LLM-запросы не выполнялись. Следующий шаг — интеграция с T8 и ревью.
- 2026-11-05 14:19 (UTC+5) — начата/возобновлена работа над T2, T4, T7–T12, T14–T15: Codex выполняет существующий план без Docker. Разделение: агент ядра — mutate/selector/audit, агент API — internal/api и go.mod/go.sum, агент UI — web; ведущий — hints, cmd/server, Makefile и интеграция. Ruling: работаем в текущей feature-ветке с непересекающимися файлами и сохраняем аудит; новый worktree не требуется. Local разрешён только для встроенного textkit, пользовательский импорт через local запрещён. Ruling: студенческий отчёт содержит только сводку без мутаций/исходного текста (Global Constraints выше общего ReportView); T5 и Docker-критерии T8/T13 остаются открыты.
- 2026-11-05 14:12 (UTC+5) — AUDIT-1 завершена: ведущий независимо воспроизвёл ошибку отката Store временным Go overlay-тестом `TestStateAuditRollback`: после отклонённого Update результат в памяти `fail`, на диске `pass`. Штатный `make test`, отдельный textkit и `go vet ./...` успешны; `make build` неуспешен из-за отсутствующего `cmd/server`; Docker и сквозной UI/публичный сценарий не проверены. Изменён только `plans.md`; исходные требования, 19 Task и все чекбоксы сохранены (проверено сравнением с HEAD), `git diff --check` успешен. Следующие действия: исправить выявленные дефекты, подтвердить T5 в Docker, реализовать T8 и T9–T12, затем выполнить T13. Коммит не создавался.
- 2026-11-05 14:07 (UTC+5) — AUDIT-1 требует проверки: завершены независимые read-only ревью T2–T5 и T6–T8/T14. Сверены строки T0–T18 без изменения исходных требований и чекбоксов. `make test` прошёл (selector 101,510 с), `(cd examples/textkit && go test ./...)` и `go vet ./...` прошли с Go 1.27.3 из `~/.local/go/bin`, временным GOCACHE и отключённой загрузкой модулей. `make build` завершился ошибкой: нет `cmd/server`. Docker-тесты не запускались: Docker не найден. Наблюдения ревью: неполная копия результатов прогонов в `internal/audit/store.go:129`; полный проект на каждого кандидата в `internal/selector/select.go:125`; лимиты импорта после clone и отсутствие ограничения внутренних HTTPS-адресов в `internal/project/import.go`; логические номера строк //line в `internal/mutate/mutate.go:89`. Проверка достоверности pass в раннере требует отдельного теста; воспроизведение обхода не выполнялось. Осталось независимо подтвердить Store и проверить diff; код приложения не изменён.
- 2026-11-05 14:02 (UTC+5) — начата работа над AUDIT-1: исполнитель — Codex, ветка `codex/shared-agent-instructions`. Исходное рабочее дерево чистое, HEAD `628d034`. Проверяется текущее состояние без изменений кода; `plans.md` ведёт только ведущий агент. Go и Docker не найдены в PATH; следующий шаг — проверить доступные установки и независимо сверить реализацию с требованиями.
- 2026-11-05 13:39 (UTC+5) — DOCS-1: начата работа над общими инструкциями и учётом статусов; исполнитель — Codex, ветка `codex/shared-agent-instructions`.
- 2026-11-05 13:42 (UTC+5) — DOCS-1 требует проверки: созданы `AGENTS.md` и `CLAUDE.md`, обновлён блок статусов в `plans.md`. Загрузка инструкций сверена через Context7 и официальную документацию Codex/Claude Code; `git diff --check` пройден. Осталось проверить сохранность требований и чекбоксов плана, ссылки и независимое ревью.
- 2026-11-05 13:44 (UTC+5) — DOCS-1 завершена: изменены только `AGENTS.md`, `CLAUDE.md` и `plans.md`. `git diff --check` и проверка структуры Python пройдены: исходный план от «Цель» до конца сохранён, все 19 задач на месте, импорт точный, размер `AGENTS.md` меньше 32 КиБ, локальные пути существуют. Независимое ревью — без замечаний. Тесты приложения не запускались: изменена только документация. Изменения локальные; для другого клона их нужно передать через Git, затем открыть новую сессию агента.

---

**Цель:** к 15:00 6 ноября 2026 года работающий по публичной ссылке MVP: аудитор импортирует Go-проект, система выдаёт студенту 3 проверенные мутации на 10 минут, аудитор видит всё вживую, в конце строится отчёт.

**Архитектура:** один Go-процесс (`cmd/server`): HTTP + WebSocket + встроенный React-интерфейс. Мутации строятся через `go/ast` с заменой байтов по смещениям. Чужой код выполняется только в Docker-контейнере `lcme-runner` без сети, где `cmd/runner` собирает тестовый бинарник один раз и запускает каждый тест отдельным процессом. Состояние хранится в памяти и в JSON-файлах.

**Стек:** Go 1.24+ (stdlib + `github.com/coder/websocket`), Docker, React 18 + TypeScript + Vite + `@monaco-editor/react` + `react-router-dom` + `qrcode.react`, OpenRouter (Chat Completions в формате OpenAI), WSL2 + ngrok.

**Spec:** `README.md`, разделы «Решение», «Архитектура», «Безопасность», «Ограничения MVP». Читать вместе с этим планом.

**Разработчик:** один человек. Время указано по Караганде (UTC+5). Коммит в `main` после каждой задачи.

---

## Global Constraints

- Ровно **3** мутации в задании. Сессия длится ровно **10 минут**, дедлайн хранит и проверяет сервер.
- Оригинал должен пройти все тесты **3 раза подряд** (`BaselineRuns = 3`), иначе подготовка отклоняется.
- Мутация попадает в задание, только если на ней падает хотя бы один тест. Тройка проходит проверку всех **8 комбинаций**.
- Каждый тест запускается отдельным процессом: `-test.run '^Name$' -test.count 1 -test.timeout 2s`. Таймаут процесса теста 5 с.
- Таймаут контейнера: **180 с** на пакет подготовки, **60 с** на прогон студента. Вывод теста обрезается до **4096 байт**, stdout раннера читается не больше **4 МиБ**.
- Флаги `docker run` (все обязательны): `--rm --network none --cap-drop ALL --security-opt no-new-privileges -u 65534:65534 --cpus 2 --memory 1g --pids-limit 512 --name lcme-<random> --mount type=bind,src=<batch>,dst=/work,readonly`.
- Окружение раннера (и в Docker, и локально): `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOFLAGS=-mod=readonly`.
- Импорт: только `https://`, URL не начинается с `-`, `git clone --depth 1 --no-tags --single-branch -- <url> <dst>`, `GIT_TERMINAL_PROMPT=0`, таймаут **60 с**, не больше **2000** файлов и **20 МиБ**. Симлинки и `.git` пропускаются, в `go.mod` нет `require`.
- Студенту никогда не отдаются: список мутаций, оригинальный код, подсказки, `killingTests`, `.git`. Файлы `*_test.go` у студента только для чтения, перед каждым прогоном сервер подставляет канонические копии.
- Максимальный размер одного файла, который присылает студент, **256 КиБ**.
- Ключ OpenRouter есть только в окружении сервера: не передаётся в контейнер и клиенту. В LLM уходят только ±3 строки вокруг мутации.
- Интерфейс на русском языке.
- Внешние Go-зависимости: только `github.com/coder/websocket`.

## Review Focus

Пять ситуаций, которые не следуют прямо из требований, но скорее всего встретятся на демо. Для каждой есть тест в задаче-владельце.

1. **Бесконечный цикл или огромный вывод в коде студента во время сессии.** Прогон возвращает `timeout` не позже чем через ~10 с, сервер не блокируется, повторное «Тесты» во время прогона получает `409`. Тесты: Task 3 (`TestLocalTimeout`), Task 8 (`TestRunConcurrent409`).
2. **Обрыв WebSocket или перезапуск туннеля.** Клиент переподключается и получает снимок с тем же `deadline`, таймер не сбрасывается. Тест: Task 8 (`TestReconnectSnapshotSameDeadline`).
3. **Перезапуск сервера посреди сессии.** Сессия поднимается из JSON. Если дедлайн прошёл, пока сервер лежал, сессия финализируется на первом тике. Тесты: Task 7 (`TestStoreRoundTrip`), Task 8 (`TestExpiredOnLoadFinalized`).
4. **Приватный или несуществующий репозиторий.** Понятная ошибка на русском, без зависания на вводе пароля. Тесты: Task 6 (`TestGitCloneFailsFast`, `TestUserMessages`).
5. **`go.mod` с `go 1.99` или с `require`.** Отказ импорта с объяснением вместо непонятной ошибки сборки. Тесты: Task 6 (`TestImportRejectsNewGo`, `TestImportRejectsRequire`).

---

## Расписание

| Время | Фаза | Задачи |
|---|---|---|
| 05.11 07:30–08:15 | 0. Окружение и каркас | Task 0, 1 |
| 08:15–11:00 | 1. Движок | Task 2, 3, 4 |
| 11:00–13:00 | 2. Песочница и импорт | Task 5, 6 |
| 13:00–13:30 | перерыв | |
| 13:30–16:30 | 3. Сессии и API | Task 7, 8 |
| 16:30–21:30 | 4. Интерфейс | Task 9, 10, 11, 12 |
| 21:30–23:00 | 5. Публичная ссылка, сквозной прогон | Task 13 — **контрольная точка MVP** |
| 23:00–01:30 | 6. Желательное | Task 14, 15 |
| 01:30–07:00 | сон | |
| 06.11 07:00–10:30 | 7. Живой тест, исправления, **заморозка в 10:30** | Task 16 |
| 10:30–13:00 | 8. Питч | Task 17 |
| 13:00–14:00 | 9. Сдача формы (крайний срок 15:00) | Task 18 |
| 14:00–17:00 | Ноутбук включён, репетиция, в 17:00 открытие | |

### Что урезать, если отстаёте

1. В 16:30 Task 8 не готова → живой diff у аудитора заменить обновлением снимка раз в 3 с (тот же WebSocket, без событий `file`).
2. В 21:30 Task 11 не готова → аудитор видит обычный read-only Editor текущего файла вместо DiffEditor.
3. В 23:00 нет публичной ссылки → всё остальное стоп, только Task 13.
4. Подсказки (Task 14) → только шаблоны. QR (Task 15) → просто ссылка. Tailscale → ngrok.

---

## Структура файлов

```
go.mod                         module lcme
Makefile                       image, web, build, run, test, test-docker, dev
.env.example  .gitignore       (data/, work/, bin/, web/dist/*, !web/dist/.gitkeep, web/node_modules/, .env)
cmd/server/main.go             конфиг из env, сборка зависимостей, http.ListenAndServe
cmd/runner/main.go             раннер внутри песочницы
internal/mutate/mutate.go      Generate, Apply
internal/sandbox/result.go     TestResult, VariantResult, ParseResults
internal/sandbox/runner.go     Runner, Batch, WriteVariant
internal/sandbox/local.go      Local
internal/sandbox/docker.go     Docker
internal/sandbox/testdata/     loop/, net/, whoami/ — фикстуры
internal/selector/select.go    Prepare, Plan, Progress
internal/selector/rank.go      Candidate, Score
internal/selector/independent.go  Independent
internal/project/import.go     Importer, Source, Project, ошибки, UserMessage
internal/project/git.go        GitClone
internal/audit/session.go      Session и её методы
internal/audit/report.go       Report, BuildReport
internal/audit/store.go        Store
internal/hints/hints.go        Hint, Input, Hinter, Template, Fallback
internal/hints/openrouter.go   OpenRouter
internal/api/server.go         Server, роуты, задача подготовки, тик таймера
internal/api/views.go          StudentView, AuditorView
internal/api/ws.go             хаб WebSocket
sandbox/Dockerfile             образ lcme-runner
examples/textkit/              демо-проект
web/embed.go                   package web; //go:embed all:dist
web/src/{main.tsx,api.ts,ws.ts,types.ts,styles.css}
web/src/pages/{Home,Auditor,Student}.tsx
web/src/components/{Timer,TestResults,FileTree,ReportView,MutationCard}.tsx
```

---

## Фаза 0. Окружение и каркас

### Task 0: Окружение на ноутбуке (без кода)

- [ ] **Step 1:** WSL2 + Ubuntu: `wsl -l -v` показывает Ubuntu с VERSION 2.
- [ ] **Step 2:** Docker Desktop → Settings → Resources → WSL Integration → Ubuntu включена. В WSL: `docker run --rm hello-world` проходит.
- [ ] **Step 3:** В WSL: `go version` ≥ 1.24, `node -v` ≥ 20, `make --version`, `git --version`.
- [ ] **Step 4:** Найти последний стабильный тег `golang:<версия>-alpine` и выполнить `docker pull golang:<версия>-alpine`. Записать версию: она пойдёт в `sandbox/Dockerfile` как `ARG GO_VERSION`.
- [ ] **Step 5:** ngrok: зарегистрироваться, `ngrok config add-authtoken <token>`, `ngrok http 8080` показывает постоянный домен `*.ngrok-free.app`. Записать домен.
- [ ] **Step 6:** Отключить сон ноутбука при питании от сети. Клонировать репозиторий **в файловую систему WSL** (`~/lcme`), а не в `/mnt/c`: так сборка и bind-mount работают быстрее.

### Task 1: Каркас репозитория и демо-проект

**Files:**
- Create: `go.mod`, `Makefile`, `.gitignore`, `.env.example`, `web/dist/.gitkeep`
- Create: `examples/textkit/go.mod` (`module example.com/textkit`, `go 1.22`), `examples/textkit/textkit.go`, `examples/textkit/textkit_test.go`

**Interfaces:**
- Produces: демо-проект `examples/textkit` с 5 функциями и 5 тестами. На нём держатся тесты Task 3, 4, 6, 8.

- [x] **Step 1:** `examples/textkit/textkit.go` — пакет `textkit`, только `strings` из stdlib:
  - `Max(nums []int) int` — `m := nums[0]`, цикл `for i := 1; i < len(nums); i++`, `if nums[i] > m`.
  - `IsPrime(n int) bool` — `if n < 2 { return false }`, цикл `for i := 2; i*i <= n; i++`, `if n%i == 0`.
  - `Capitalize(s string) string` — `strings.Fields`, `if len(w) > 0`, `strings.ToUpper(w[:1]) + strings.ToLower(w[1:])`, `strings.Join(words, " ")`.
  - `AtoiBase(s, base string) int` — `if len(base) < 2 { return 0 }`, `idx := strings.IndexRune(base, r)`, `if idx == -1 { return 0 }`, `res = res*len(base) + idx`.
  - `WindowSums(nums []int, k int) []int` — `if k <= 0 || k > len(nums) { return nil }`, скользящая сумма: `if i >= k { sum -= nums[i-k] }`, `if i >= k-1 { out = append(out, sum) }`.
- [x] **Step 2:** `examples/textkit/textkit_test.go` — по одной табличной функции `TestMax`, `TestIsPrime`, `TestCapitalize`, `TestAtoiBase`, `TestWindowSums` с `t.Errorf` (не `Fatal`). Обязательные случаи:
  - `Max([]int{1})=1`, `{3,9,2}=9`, `{-5,-2,-9}=-2`, `{4,4,1}=4`, `{1,2,3,10}=10`;
  - `IsPrime`: −7, 0, 1, 4, 9, 25 → false; 2, 3, 13, 97 → true;
  - `Capitalize`: `"hello world"→"Hello World"`, `"gO pRO"→"Go Pro"`, `"a"→"A"`, `""→""`;
  - `AtoiBase`: `("101","01")=5`, `("ff","0123456789abcdef")=255`, `("z","01")=0`, `("1","0")=0`, `("","01")=0`;
  - `WindowSums`: `([1,2,3,4],2)=[3,5,7]`, `([5],1)=[5]`, `([1,2],3)=nil`, `([1,2,3],0)=nil`, `([1,2,3],3)=[6]`.
- [x] **Step 3:** Run: `cd examples/textkit && go test ./...` → `ok`.
- [x] **Step 4:** Корневой `go.mod`: `module lcme`, `go 1.24`. `Makefile` с целями из README (`image`, `web`, `build`, `run`, `test`, `test-docker`, `dev`). Цель `build` собирает `bin/lcme` и `bin/lcme-runner`. `.env.example` со всеми переменными из таблицы в README.
- [ ] **Step 5:** Run: `make test` → `go: warning: "./..." matched no packages` (пакетов ещё нет; `examples/textkit` — отдельный модуль и в `./...` корня не входит, так и задумано).
- [ ] **Step 6:** Commit: `git add -A && git commit -m "chore: каркас репозитория и демо-проект textkit"`.

---

## Фаза 1. Движок

### Task 2: Мутации (`internal/mutate`)

**Files:**
- Create: `internal/mutate/mutate.go`
- Test: `internal/mutate/mutate_test.go`

**Interfaces:**
- Produces:
  ```go
  type Kind string // "boundary" | "negate" | "logical" | "arith" | "incdec" | "constant"
  type Mutation struct {
      ID         string `json:"id"`   // "c1".."cN" по порядку (файл, смещение)
      File       string `json:"file"` // путь со слэшами относительно корня проекта
      Line       int    `json:"line"`
      Func       string `json:"func"`
      Kind       Kind   `json:"kind"`
      Start      int    `json:"start"` // байтовое смещение в оригинале
      End        int    `json:"end"`
      Orig       string `json:"orig"`
      Repl       string `json:"repl"`
  }
  func Generate(files map[string][]byte) ([]Mutation, error) // ключи — не-тестовые .go-файлы
  func Apply(files map[string][]byte, ms []Mutation) (map[string][]byte, error) // копия всех файлов
  var ErrOverlap = errors.New("mutations overlap")
  ```

- [x] **Step 1: Тесты**

```go
const src = "package a\n\nfunc F(n int) bool {\n\tfor i := 0; i < n; i++ {\n\t\tif i == 3 && n > 1 {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n"

func TestGenerateFindsOperators(t *testing.T) {
    files := map[string][]byte{"a.go": []byte(src)}
    ms, err := Generate(files)
    // err == nil; len(ms) == 8
    // ms[0]: ID "c1", Kind "constant", Orig "0", Repl "1", Line 4, Func "F"
    // для каждого m: string(files[m.File][m.Start:m.End]) == m.Orig
    // виды: constant×3 ("0","3","1"), boundary×2 ("<"→"<=", ">"→">="), incdec×1 ("++"→"--"), negate×1 ("=="→"!="), logical×1 ("&&"→"||")
}
func TestApplyChangesExactlyOneLine(t *testing.T)   // Apply(ms[1]) → ровно 1 строка отличается от src
func TestApplyRejectsOverlap(t *testing.T)          // Apply(files, []Mutation{ms[0], ms[0]}) → errors.Is(err, ErrOverlap)
func TestGenerateSkipsBigConstants(t *testing.T)    // литерал 100 не мутируется, 99 мутируется в 100
```

- [ ] **Step 2:** Run: `go test ./internal/mutate/...` → FAIL (`undefined: Generate`).
- [x] **Step 3:** Реализовать `Generate`. `go/parser.ParseFile` по каждому файлу в отсортированном порядке, обход `*ast.FuncDecl.Body` через `ast.Inspect`:
  - `BinaryExpr`: `< ↔ <=`, `> ↔ >=` (boundary); `== ↔ !=` (negate); `&& ↔ ||` (logical); `+ ↔ -` (arith). Позиция — `OpPos`, длина — `len(Op.String())`.
  - `IncDecStmt`: `++ ↔ --`, позиция `TokPos`.
  - `BasicLit` `INT` со значением `< 100`: `v → v+1`.

  Смещение — `fset.Position(pos).Offset`. После сортировки по (файл, `Start`) присвоить `ID = "c" + номер`. Реализовать `Apply`: внутри файла заменять с конца к началу, пересечение диапазонов → `ErrOverlap`.
- [x] **Step 4:** Run: `go test ./internal/mutate/...` → PASS.
- [ ] **Step 5:** Commit: `feat(mutate): кандидаты через go/ast и замена по смещениям`.

### Task 3: Протокол песочницы, раннер и `Local` (`internal/sandbox`, `cmd/runner`)

**Files:**
- Create: `internal/sandbox/result.go`, `internal/sandbox/runner.go`, `internal/sandbox/local.go`, `cmd/runner/main.go`
- Create: `internal/sandbox/testdata/loop/{go.mod,loop.go,loop_test.go}` (`func Spin() { for {} }`, тест его вызывает)
- Test: `internal/sandbox/local_test.go`

**Interfaces:**
- Consumes: `mutate.Generate`, `mutate.Apply` (Task 2).
- Produces:
  ```go
  type Status string // "pass" | "fail" | "timeout"
  type TestResult struct { Pkg, Name string; Status Status; Panicked bool; Output string } // JSON: pkg, name, status, panicked, output
  func (t TestResult) Key() string // Pkg + ":" + Name, например ".:TestMax"
  type VariantResult struct { ID string; BuildError string; Tests []TestResult; DurationMs int64 }
  func (v VariantResult) Passed(key string) bool          // false, если теста нет
  func (v VariantResult) AllPassed(keys []string) bool
  func (v VariantResult) Keys() []string                  // отсортированные ключи тестов
  type Batch struct { Dir string; IDs []string }          // Dir/<id>/ — полная копия проекта
  type Runner interface {
      Run(ctx context.Context, b Batch) ([]VariantResult, error)
      GoVersion(ctx context.Context) (string, error)     // например "go1.26.2"
  }
  func WriteVariant(batchDir, id string, files map[string][]byte) error
  func ParseResults(r io.Reader, limit int64) ([]VariantResult, error) // одна строка JSON = один VariantResult
  func NewLocal(runnerBin string) *Local                  // Local реализует Runner
  const TestTimeout = 2 * time.Second
  const OutputCap = 4096
  ```
- CLI `lcme-runner -dir <batch> -ids a,b,c [-test-timeout 2s]` → в stdout по одной строке `VariantResult` на вариант, в порядке `ids`.

- [ ] **Step 1: Тесты.** `TestMain` собирает раннер: `go build -o <tmp>/lcme-runner lcme/cmd/runner`. Помощник `variant(t, files, ms...)` пишет `WriteVariant` в `t.TempDir()`.

```go
func TestLocalBaselineAllPass(t *testing.T) // textkit без мутаций: BuildError == "", 5 тестов, все "pass"
func TestLocalPanicIsolated(t *testing.T)   // мутация Func=="Max", Orig=="<" (в условии цикла):
                                            // ".:TestMax" fail && Panicked; остальные 4 теста "pass"
func TestLocalBuildError(t *testing.T)      // "return m" → "return x" в Max: BuildError != "", len(Tests) == 0
func TestLocalTimeout(t *testing.T)         // testdata/loop: Status "timeout", весь Run < 8 с
func TestLocalOutputCapped(t *testing.T)    // тест печатает 1 МБ и падает: len(Output) <= OutputCap
```

- [ ] **Step 2:** Run: `go test ./internal/sandbox/...` → FAIL.
- [ ] **Step 3:** Реализовать `cmd/runner`. Это алгоритм, который не следует из сигнатур:

```text
для каждого id из -ids:
  dir := <batch>/<id>
  pkgs := каталоги с *_test.go (WalkDir; пропускать testdata, vendor, имена на "." и "_")
  для каждого pkg (по алфавиту):
    go test -c -o $TMPDIR/<id>/<n>.test ./<pkg>   (cwd = dir)
      ошибка → BuildError = вывод (обрезанный до OutputCap), перейти к следующему id
    names := <bin> -test.list '^Test'   (строки, начинающиеся с "Test")
    для каждого name:
      <bin> -test.run '^'+name+'$' -test.count 1 -test.timeout 2s  (cwd = dir/pkg, ctx-таймаут 5s)
      exit 0 → pass; вывод содержит "panic: test timed out" или истёк ctx → timeout; иначе fail
      Panicked = вывод содержит "panic: " и это не таймаут
      Output = последние OutputCap байт вывода
  печатать VariantResult одной строкой JSON
```

  `Local.Run` запускает `runnerBin` с окружением `GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-mod=readonly` и парсит stdout через `ParseResults` с лимитом 4 МиБ. `Local.GoVersion` = `go env GOVERSION`.
- [ ] **Step 4:** Run: `go test ./internal/sandbox/...` → PASS.
- [ ] **Step 5:** Commit: `feat(sandbox): раннер с запуском тестов по отдельности и Local runner`.

### Task 4: Отбор тройки (`internal/selector`)

**Files:**
- Create: `internal/selector/independent.go`, `internal/selector/rank.go`, `internal/selector/select.go`
- Create: `internal/selector/testdata/red/` (проект с падающим тестом), `internal/selector/testdata/onefunc/` (одна функция с 3+ операторами и один тест)
- Test: `internal/selector/selector_test.go`

**Interfaces:**
- Consumes: `mutate.*` (Task 2), `sandbox.Runner`, `sandbox.WriteVariant`, `VariantResult.Passed/AllPassed/Keys` (Task 3).
- Produces:
  ```go
  type Options struct { BaselineRuns, MaxTriples, TriplesPerBatch int }
  var DefaultOptions = Options{BaselineRuns: 3, MaxTriples: 30, TriplesPerBatch: 5}
  type Progress struct { Stage string; Done, Total int } // Stage: "baseline" | "candidates" | "triples" | "done"
  type Candidate struct { M mutate.Mutation; Failing []string; Panicked, Timeout bool }
  func Score(c Candidate) int
  func Independent(r [8]sandbox.VariantResult, tests []string) (killing [3][]string, ok bool)
  type Stats struct { Candidates, Killed, Survived, BuildErrors, TriplesChecked int }
  type Plan struct {
      Mutations     []mutate.Mutation // 3 шт., ID переименованы в "m1","m2","m3" в порядке (файл, смещение)
      KillingTests  [][]string        // [i] — тесты, падающие ровно тогда, когда есть мутация i
      BaselineTests []string
      Mutated       map[string][]byte // все файлы проекта с тремя мутациями (json:"-")
      Stats         Stats
  }
  func Prepare(ctx context.Context, files map[string][]byte, goFiles map[string][]byte,
      r sandbox.Runner, workDir string, opt Options, onProgress func(Progress)) (*Plan, error)
  var ErrBaselineRed = errors.New("baseline is not green")
  var ErrNoTriple = errors.New("no independent triple")
  ```
  `files` — все файлы проекта, `goFiles` — только не-тестовые `.go` (вход `mutate.Generate`).

- [x] **Step 1: Чистые тесты**

```go
// mk(fails func(mask int, test string) bool) строит [8]VariantResult по тестам "A","B","C"
func TestIndependentOK(t *testing.T)         // A падает ⇔ бит0, B ⇔ бит1, C ⇔ бит2 → ok, killing == [[A] [B] [C]]
func TestIndependentMasked(t *testing.T)     // B ещё и падает при бите0 (маскирование паникой) → ok == false
func TestIndependentSharedTest(t *testing.T) // только один тест, падает при любой мутации → ok == false
func TestIndependentBuildError(t *testing.T) // маска 3 с BuildError → ok == false
func TestScoreOrder(t *testing.T)            // boundary > arith > constant; boundary с Panicked < boundary без
```

- [ ] **Step 2:** Run: `go test ./internal/selector/ -run 'Independent|Score'` → FAIL.
- [x] **Step 3:** Реализовать `Independent`: `killing[i]` = тесты `t`, для которых при всех `mask` в 0..7 `r[mask].Passed(t) == (mask&(1<<i) == 0)`; `ok` = все три списка не пусты. Реализовать `Score`: вес вида boundary/negate/logical = 10, incdec/arith = 6, constant = 2; `Panicked` −8; `Func != "main"` +2.
- [ ] **Step 4:** Run → PASS.
- [x] **Step 5: Интеграционные тесты** (`Local` из Task 3 на `examples/textkit`):

```go
func TestPrepareTextkit(t *testing.T)       // len(Mutations)==3; Func у всех разные; каждый KillingTests[i] не пуст;
                                            // Mutated отличается от оригинала ровно в 3 строках; ID == m1,m2,m3;
                                            // у выбранных кандидатов Panicked == false
func TestPrepareBaselineRed(t *testing.T)   // testdata/red → errors.Is(err, ErrBaselineRed)
func TestPrepareNoTriple(t *testing.T)      // testdata/onefunc → errors.Is(err, ErrNoTriple)
```

- [x] **Step 6:** Реализовать `Prepare`:
  1. `BaselineRuns` вариантов оригинала в одном `Batch`: все зелёные и с одинаковым набором тестов, иначе `ErrBaselineRed`. Текст ошибки перечисляет упавшие тесты.
  2. `Generate` → каждый кандидат отдельным вариантом в одном `Batch`. Отбросить сборочные ошибки, таймауты и выживших.
  3. Отсортировать по `Score`, взять top-12, перебирать тройки с разными `Func` по убыванию суммы `Score`, пропуская тройки, где у двух кандидатов совпадает `Failing`.
  4. Для каждой тройки нужны маски 3, 5, 6, 7 (маски 0, 1, 2, 4 уже известны). По `TriplesPerBatch` троек в одном `Batch`, не больше `MaxTriples`. Первая тройка, у которой `Independent` дал ok, становится результатом. Если такой нет, `ErrNoTriple`.

  `onProgress` вызывать после каждого `Batch`.
- [x] **Step 7:** Run: `go test ./internal/selector/...` → PASS.
- [ ] **Step 8:** Commit: `feat(selector): отбор тройки с проверкой 8 комбинаций`.

---

## Фаза 2. Песочница и импорт

### Task 5: Docker-песочница (`sandbox/Dockerfile`, `internal/sandbox/docker.go`)

**Files:**
- Create: `sandbox/Dockerfile`, `internal/sandbox/docker.go`
- Create: `internal/sandbox/testdata/net/` (тест: `net.DialTimeout("tcp", "1.1.1.1:443", time.Second)` должен вернуть `nil`), `internal/sandbox/testdata/whoami/` (тест: `os.Getuid() == 65534`)
- Test: `internal/sandbox/docker_test.go` (выполняется только при `LCME_DOCKER=1`, иначе `t.Skip`)

**Interfaces:**
- Consumes: `Runner`, `Batch`, `ParseResults` (Task 3).
- Produces: `func NewDocker(image string, timeout time.Duration) *Docker` (реализует `Runner`).

- [ ] **Step 1:** `sandbox/Dockerfile`: многоэтапная сборка.
  - Этап `build` (`golang:${GO_VERSION}-alpine`) собирает `/out/lcme-runner` из `./cmd/runner` с `CGO_ENABLED=0`.
  - Финальный этап на `golang:${GO_VERSION}-alpine`: копирует бинарник в `/usr/local/bin/`, задаёт `ENV GOCACHE=/opt/gocache GOPATH=/tmp/go HOME=/tmp GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-mod=readonly`.
  - Прогрев кэша: модуль-пустышка импортирует `fmt strings strconv sort os bufio unicode errors math bytes io time regexp testing reflect`, затем `go test -c -o /dev/null .` и `chown -R 65534:65534 /opt/gocache`.
- [ ] **Step 2: Тесты**

```go
func TestDockerBaselineAllPass(t *testing.T)  // как TestLocalBaselineAllPass, через NewDocker("lcme-runner:latest", 180*time.Second)
func TestDockerPanicIsolated(t *testing.T)    // как в Task 3
func TestDockerNoNetwork(t *testing.T)        // testdata/net: тест "fail" (сети нет)
func TestDockerRunsAsNobody(t *testing.T)     // testdata/whoami: тест "pass"
func TestDockerTimeoutKillsContainer(t *testing.T) // ctx на 1 с + testdata/loop: Run вернул ошибку,
                                                   // через 2 с `docker ps -q --filter name=lcme-` пусто
func TestDockerGoVersion(t *testing.T)        // строка начинается с "go1."
```

- [ ] **Step 3:** Run: `make image && LCME_DOCKER=1 go test ./internal/sandbox/ -run Docker` → FAIL (`undefined: NewDocker`).
- [ ] **Step 4:** Реализовать `Docker.Run`: `docker run` со всеми флагами из Global Constraints, `--name lcme-<16 hex>`, команда `lcme-runner -dir /work -ids <ids>`. stdout через `ParseResults` (лимит 4 МиБ). При отмене ctx выполнить `docker kill lcme-<id>`. `GoVersion` = `docker run --rm <image> go env GOVERSION`.
- [ ] **Step 5:** Run: `LCME_DOCKER=1 go test ./internal/sandbox/...` → PASS. Замерить время `TestDockerBaselineAllPass`: должно быть < 5 с с прогретым кэшем. Если больше, проверить, что `GOCACHE` указывает в образ.
- [ ] **Step 6:** Commit: `feat(sandbox): Docker-песочница без сети и привилегий`.

### Task 6: Импорт проекта (`internal/project`)

**Files:**
- Create: `internal/project/import.go`, `internal/project/git.go`
- Test: `internal/project/import_test.go`

**Interfaces:**
- Produces:
  ```go
  type Source struct { GitURL string `json:"gitUrl,omitempty"`; Example string `json:"example,omitempty"` }
  type Project struct {
      Source      Source
      Module      string
      GoDirective string            // "1.22"
      Files       map[string][]byte // все обычные файлы, пути со слэшами, без .git
      SourceFiles []string          // не-тестовые .go, по алфавиту
      TestFiles   []string          // *_test.go, по алфавиту
  }
  func (p *Project) GoFiles() map[string][]byte // только SourceFiles — вход для selector.Prepare
  type Limits struct { MaxFiles int; MaxBytes int64; CloneTimeout time.Duration }
  var DefaultLimits = Limits{MaxFiles: 2000, MaxBytes: 20 << 20, CloneTimeout: 60 * time.Second}
  type Cloner func(ctx context.Context, url, dst string) error
  func GitClone(ctx context.Context, url, dst string) error
  type Importer struct { ExamplesDir, WorkDir string; Clone Cloner; Limits Limits; MaxGo string } // MaxGo: "1.26"
  func (im *Importer) Import(ctx context.Context, src Source) (*Project, error)
  var ErrBadURL, ErrClone, ErrNoGoMod, ErrExternalDeps, ErrNoTests, ErrTooLarge, ErrGoTooNew, ErrUnknownExample error
  func UserMessage(err error) string // текст для интерфейса на русском
  ```

- [ ] **Step 1: Тесты** (фейковый `Cloner` копирует фикстуру из `t.TempDir()`):

```go
func TestImportExample(t *testing.T)            // Source{Example:"textkit"}: Module "example.com/textkit",
                                                // SourceFiles ["textkit.go"], TestFiles ["textkit_test.go"]
func TestImportRejectsBadURL(t *testing.T)      // "http://x/y", "file:///etc", "--upload-pack=x", "git@github.com:a/b", "" → ErrBadURL
func TestImportStripsGitAndSymlinks(t *testing.T) // фикстура с .git/config и симлинком secret -> /etc/passwd:
                                                  // в Files нет ключей ".git/config" и "secret"
func TestImportRejectsRequire(t *testing.T)     // в go.mod есть "require github.com/x/y v1.0.0" → ErrExternalDeps
func TestImportRejectsNoTests(t *testing.T)     // нет *_test.go → ErrNoTests
func TestImportRejectsNewGo(t *testing.T)       // "go 1.99", MaxGo "1.26" → ErrGoTooNew
func TestImportLimits(t *testing.T)             // Limits{MaxFiles: 3} при 5 файлах → ErrTooLarge
func TestGitCloneFailsFast(t *testing.T)        // GitClone(ctx, "https://127.0.0.1:1/x.git", dir) → ошибка за < 5 с
func TestUserMessages(t *testing.T)             // для каждой Err* UserMessage не пуст и не содержит английского текста ошибки
```

- [ ] **Step 2:** Run: `go test ./internal/project/...` → FAIL.
- [ ] **Step 3:** Реализовать.
  - URL: `net/url.Parse`, `Scheme == "https"`, `Host != ""`, не начинается с `-`.
  - `GitClone`: `exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--no-tags", "--single-branch", "--", url, dst)` с `GIT_TERMINAL_PROMPT=0`. Ошибку оборачивать в `ErrClone` вместе с последней строкой stderr.
  - Чтение дерева: `WalkDir` + `Lstat`, только обычные файлы, пропуск `.git`. Лимиты считаются по ходу обхода.
  - `go.mod`: `module` и `go` из строк. Любая строка, начинающаяся с `require` → `ErrExternalDeps`. Версии сравнивать как числа по major.minor.
  - Пример: копия `ExamplesDir/<name>`.
- [ ] **Step 4:** Run: `go test ./internal/project/...` → PASS.
- [ ] **Step 5:** Commit: `feat(project): безопасный импорт git-репозитория и демо-проекта`.

---

## Фаза 3. Сессии и API

### Task 7: Сессия, отчёт, хранилище (`internal/audit`)

**Files:**
- Create: `internal/audit/session.go`, `internal/audit/report.go`, `internal/audit/store.go`
- Test: `internal/audit/session_test.go`, `internal/audit/report_test.go`, `internal/audit/store_test.go`

**Interfaces:**
- Consumes: `selector.Plan`, `selector.Progress` (Task 4); `project.Source`, `project.Project` (Task 6); `sandbox.VariantResult` (Task 3); `hints.Hint` (Task 14). До Task 14 объявить в `internal/hints/hints.go` только тип `Hint`.
- Produces:
  ```go
  type State string // "preparing" | "ready" | "running" | "finished" | "failed"
  const SessionDuration = 10 * time.Minute
  const MaxFileSize = 256 << 10
  type Role string  // "auditor" | "student"
  type Flag struct { ID, File string; Line int; Note string; At time.Time; MutationID string; Accepted *bool }
  type Run struct { At time.Time; Final bool; Result sandbox.VariantResult }
  type Event struct { At time.Time; Kind string; Size int } // Kind: "hidden" | "visible" | "paste"
  type Session struct {
      ID, AuditorToken, StudentToken string
      Source   project.Source
      State    State; Error string
      Progress selector.Progress
      Plan     *selector.Plan
      Hints    []hints.Hint
      Files    map[string][]byte // текущие не-тестовые файлы студента
      Tests    map[string][]byte // *_test.go и прочие файлы только для чтения
      Mutated  map[string][]byte // задание как выдано (левая сторона diff)
      CreatedAt, StartedAt, Deadline, FinishedAt time.Time
      Flags []Flag; Runs []Run; Events []Event
  }
  func New(src project.Source, now time.Time) *Session // токены: 16 случайных байт в hex (crypto/rand)
  func (s *Session) SetReady(p *project.Project, plan *selector.Plan, h []hints.Hint)
  func (s *Session) Fail(msg string)
  func (s *Session) Start(now time.Time) error
  func (s *Session) UpdateFile(path string, content []byte, now time.Time) error
  func (s *Session) AddFlag(file string, line int, note string, now time.Time) (Flag, error)
  func (s *Session) ReviewFlag(flagID, mutationID string, accepted bool) error
  func (s *Session) AddEvent(kind string, size int, now time.Time) error
  func (s *Session) AddRun(r sandbox.VariantResult, final bool, now time.Time)
  func (s *Session) Expired(now time.Time) bool
  func (s *Session) Finish(now time.Time)
  func (s *Session) RunFiles() map[string][]byte // Files + Tests (Tests перекрывают)
  var ErrBadState, ErrReadOnly, ErrUnknownFile, ErrExpired, ErrTooBig, ErrUnknownFlag, ErrUnknownMutation error

  type MutationReport struct { ID, File string; Line int; Kind mutate.Kind; Orig, Repl string
                               Detected, Explained, Fixed bool; FixedAfterSec int } // -1, если не исправлена
  type Report struct { Mutations []MutationReport; Restored bool
                       HiddenCount, PasteCount, PasteChars, DurationSec int }
  func BuildReport(s *Session) Report

  type Store struct { /* dir, mu, map */ }
  func OpenStore(dir string) (*Store, error)            // загружает все <dir>/<id>.json
  func (st *Store) Create(s *Session) error
  func (st *Store) Update(id string, fn func(*Session) error) error // блокировка + fn + запись на диск
  func (st *Store) View(id string, fn func(*Session))
  func (st *Store) ByToken(tok string) (id string, role Role, ok bool)
  func (st *Store) IDs() []string
  ```

- [x] **Step 1: Тесты**

```go
func TestStartSetsDeadline(t *testing.T)    // ready → Start(t0): State "running", Deadline == t0.Add(10*time.Minute); второй Start → ErrBadState
func TestUpdateFileRules(t *testing.T)      // "textkit_test.go" → ErrReadOnly; "nope.go" → ErrUnknownFile;
                                            // now > Deadline → ErrExpired; 300 KiB → ErrTooBig; "textkit.go" → nil и Files обновлён
func TestReportFixedByTestsNotText(t *testing.T) // KillingTests [[A] [B] [C]], финальный прогон: A и B pass, C fail,
                                                 // Files ≠ оригинал (другое решение) → Fixed [true true false], Restored false
func TestReportDetectedExplained(t *testing.T)   // отметка, привязанная к m2 и принятая → m2 Detected и Explained;
                                                 // отметка к m1 отклонена → m1 Detected, но не Explained; m3 ни то ни другое
func TestReportFixedAfterSec(t *testing.T)       // прогон на +60 с: A fail; прогон на +120 с: A pass → m1.FixedAfterSec == 120
func TestReportSignals(t *testing.T)             // события hidden×2, paste(120), paste(30) → HiddenCount 2, PasteCount 2, PasteChars 150
func TestStoreRoundTrip(t *testing.T)            // Create + Update; OpenStore(тот же dir) → поля совпадают (вкл. Files, Plan.Mutated);
                                                 // ByToken(AuditorToken) → role "auditor"; ByToken(StudentToken) → "student"
```

- [ ] **Step 2:** Run: `go test ./internal/audit/...` → FAIL.
- [x] **Step 3:** Реализовать.
  - `SetReady`: `Mutated = Plan.Mutated`. `Files` = копия `Mutated` только для не-тестовых `.go`. `Tests` = всё остальное. Задание и оригинал хранятся только на сервере.
  - `Detected` = есть отметка с этим `MutationID`. `Explained` = у неё `Accepted == true`. `Fixed` = все `KillingTests[i]` проходят в последнем прогоне с `Final == true`. `FixedAfterSec` = первый прогон, где они все прошли, считая от `StartedAt`. `Restored` = финальный прогон `AllPassed(BaselineTests)` и без `BuildError`.
  - `Store` сериализует `Session` целиком в `<dir>/<id>.json`, причём `[]byte` уходят в base64 по умолчанию. `Plan.Mutated` хранится в отдельном поле `Session.Mutated`.
- [x] **Step 4:** Run → PASS.
- [ ] **Step 5:** Commit: `feat(audit): сессия с серверным таймером, отчёт, хранение в JSON`.

### Task 8: HTTP, WebSocket, подготовка, таймер (`internal/api`, `cmd/server`)

**Files:**
- Create: `internal/api/server.go`, `internal/api/views.go`, `internal/api/ws.go`, `cmd/server/main.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: всё выше. `github.com/coder/websocket` (`go get github.com/coder/websocket`).
- Produces:
  ```go
  type PrepareFunc func(ctx context.Context, p *project.Project, onProgress func(selector.Progress)) (*selector.Plan, error)
  type Deps struct {
      Store *audit.Store; Importer *project.Importer; Runner sandbox.Runner
      Prepare PrepareFunc; Hinter hints.Hinter; Now func() time.Time
      WorkDir, PublicURL string; Static fs.FS // web.Dist; nil в тестах
  }
  func New(d Deps) *Server
  func (s *Server) Handler() http.Handler
  func (s *Server) Tick(ctx context.Context) // финализирует истёкшие сессии; main вызывает раз в секунду
  ```
  REST (`{tok}` — токен аудитора или студента; чужая роль → 403):
  | Метод и путь | Роль | Результат |
  |---|---|---|
  | `POST /api/audits` `{"gitUrl"}` или `{"example":"textkit"}` | — | 201 `{"auditorUrl": "/a/<tok>"}`, подготовка идёт в фоне |
  | `GET /api/a/{tok}` | аудитор | `AuditorView` |
  | `POST /api/a/{tok}/flags/{flagId}` `{"mutationId","accepted"}` | аудитор | 204 |
  | `POST /api/a/{tok}/finish` | аудитор | 202 (финальный прогон в фоне) |
  | `GET /api/s/{tok}` | студент | `StudentView` |
  | `POST /api/s/{tok}/start` | студент | 204 |
  | `POST /api/s/{tok}/run` | студент | 202; если прогон уже идёт → 409 |
  | `POST /api/s/{tok}/flags` `{"file","line","note"}` | студент | 201 |
  | `POST /api/s/{tok}/finish` | студент | 202 |
  | `GET /ws/{tok}` | обе | WebSocket |
  | `GET /healthz` | — | 200 `ok` |
  | иначе | — | файл из `Static` или `index.html` (SPA) |

  WebSocket. Сервер → клиент: `{"type":"snapshot","view":<View>}` при подключении и при любом изменении; аудитору при правке студента `{"type":"file","path","content"}`. Клиент студента → сервер: `{"type":"file","path","content"}`, `{"type":"event","kind","size"}`. Ошибка → `{"type":"error","message"}`.

  `StudentView`: `state, serverNow, deadline?, files map[path]string, readOnly []string, runs []RunView, flags []{id,file,line,note}, report?` (`report` только в состоянии `finished`). Полей `mutations`, `hints`, `killingTests`, `mutated` нет.
  `AuditorView`: всё из `StudentView` плюс `progress, error, studentUrl, mutations, killingTests, hints, mutated, flags (с mutationId/accepted), events, stats`.

- [x] **Step 1: Тесты** (`httptest.Server`; `Prepare` — фейк, сразу возвращающий план на textkit с мутациями `m1..m3`; `Runner` — фейк с настраиваемым ответом и каналом-блокировкой; `Now` — управляемые часы):

```go
func TestCreateAuditExampleBecomesReady(t *testing.T) // POST {"example":"textkit"} → 201; GET аудитора в течение 2 с даёт state "ready", studentUrl непустой
func TestStudentViewHasNoAnswers(t *testing.T)        // в JSON студента нет ключей "mutations","hints","killingTests","mutated","orig";
                                                      // files["textkit.go"] == Mutated["textkit.go"]
func TestStudentCannotEditTests(t *testing.T)         // WS {"type":"file","path":"textkit_test.go"} → {"type":"error"}, Tests не изменились
func TestRunUsesCanonicalTests(t *testing.T)          // фейковый Runner видит в Batch textkit_test.go побайтно равный каноническому
func TestRunConcurrent409(t *testing.T)               // первый run блокируется в фейковом Runner; второй POST run → 409
func TestReconnectSnapshotSameDeadline(t *testing.T)  // start; WS #1 → deadline D; закрыть; WS #2 → deadline == D
func TestAuditorGetsLiveFile(t *testing.T)            // студент шлёт file по WS → аудитор получает {"type":"file"} с тем же content
func TestTimerFinalizes(t *testing.T)                 // start; часы +10 мин 1 с; Tick → state "finished", последний Run.Final == true
func TestExpiredOnLoadFinalized(t *testing.T)         // в Store лежит running-сессия с прошедшим deadline → New + Tick → "finished"
func TestPrepareFailureShownToAuditor(t *testing.T)   // Prepare вернул ErrNoTriple → state "failed", error на русском
```

- [ ] **Step 2:** Run: `go test ./internal/api/...` → FAIL.
- [x] **Step 3:** Реализовать `Server`.
  - Маршруты на `http.ServeMux` с шаблонами Go 1.22 (`"POST /api/s/{tok}/run"`).
  - Подготовка в горутине: `Importer.Import` → `Prepare` (прогресс в `Store.Update` + рассылка) → `Hinter.Hints` с таймаутом 20 с → `SetReady`. Любая ошибка → `Fail(project.UserMessage(err))` или текст для `ErrBaselineRed` / `ErrNoTriple`.
  - Прогон: `RunFiles()` → `sandbox.WriteVariant(<work>/<id>/run-<n>, "v", …)` → `Runner.Run` с таймаутом 60 с → `AddRun` → рассылка. Не больше одного прогона на сессию одновременно.
  - Хаб: `map[sessionID][]*conn` под мьютексом, у каждого соединения своя роль. `Tick` финализирует только сессии, у которых `Expired`.
  - `cmd/server/main.go`: чтение переменных из README. `LCME_RUNNER=local` пишет предупреждение в лог. `MaxGo` берётся из `Runner.GoVersion` на старте (`"go1.26.2"` → `"1.26"`). `Prepare` = `func(ctx, p, cb) { return selector.Prepare(ctx, p.Files, p.GoFiles(), runner, workDir, selector.DefaultOptions, cb) }`. `Static` = `fs.Sub(web.Dist, "dist")`. Тикер раз в 1 с.
- [x] **Step 4:** Run: `go test ./internal/api/...` → PASS; `go test ./...` → PASS.
- [ ] **Step 5:** Ручная проверка: `LCME_RUNNER=docker go run ./cmd/server`. Выполнить `curl -X POST localhost:8080/api/audits -d '{"example":"textkit"}'`, затем `curl localhost:8080/api/a/<tok>` и дождаться `"state":"ready"` (с Docker < 60 с).
- [ ] **Step 6:** Commit: `feat(api): REST, WebSocket, фоновая подготовка, серверный таймер`.

---

## Фаза 4. Интерфейс

Общая проверка для Task 9–12: `cd web && npm run typecheck && npm run build` без ошибок, затем ручной сценарий из задачи. Типы в `web/src/types.ts` повторяют `StudentView`, `AuditorView`, `Report` из Task 7–8 (поля в camelCase, как в JSON).

### Task 9: Каркас фронтенда, клиенты API и WebSocket, страница «Старт»

**Files:**
- Create: `web/package.json`, `web/vite.config.ts` (прокси `/api` и `/ws` на `localhost:8080`, `ws: true`), `web/tsconfig.json`, `web/index.html`, `web/embed.go`
- Create: `web/src/main.tsx` (маршруты `/`, `/a/:tok`, `/s/:tok`), `web/src/api.ts`, `web/src/ws.ts`, `web/src/types.ts`, `web/src/styles.css`, `web/src/pages/Home.tsx`

**Interfaces:**
- Produces:
  ```ts
  // api.ts
  export function createAudit(body: {gitUrl?: string; example?: string}): Promise<{auditorUrl: string}>
  export function post(path: string, body?: unknown): Promise<Response>
  // ws.ts
  export function useSession<V>(tok: string): { view: V | null; connected: boolean; send(msg: object): void; lastFile?: {path: string; content: string} }
  // переподключение с задержкой 0.5 → 1 → 2 → 5 с; после подключения сервер сам шлёт snapshot
  // timer: export function useCountdown(deadline?: string, serverNow?: string): number // секунды до конца с поправкой на разницу часов
  ```
  `web/embed.go`: `package web`, `//go:embed all:dist`, `var Dist embed.FS`.

- [x] **Step 1:** `npm create vite@latest web -- --template react-ts` (в существующую папку), затем `npm i react-router-dom @monaco-editor/react qrcode.react`. Скрипт `"typecheck": "tsc --noEmit"`.
- [x] **Step 2:** `Home`: поле «Ссылка на git-репозиторий (https)», кнопки «Подготовить аудит» и «Попробовать на демо-проекте». После ответа переход на `auditorUrl`. Ошибка запроса выводится под формой.
- [x] **Step 3:** Run: `npm run typecheck && npm run build` → без ошибок, `web/dist/index.html` есть. `go build ./...` → OK (embed видит `dist`).
- [ ] **Step 4:** Ручная проверка: `make dev` → на `localhost:5173` нажать «демо-проект» → открывается `/a/<tok>` (пока пустая страница).
- [ ] **Step 5:** Commit: `feat(web): каркас, клиенты API и WebSocket, стартовая страница`.

### Task 10: Экран студента

**Files:**
- Create: `web/src/pages/Student.tsx`, `web/src/components/Timer.tsx`, `web/src/components/FileTree.tsx`, `web/src/components/TestResults.tsx`

- [x] **Step 1:** Состояние `ready`: правила (10 минут, без ИИ-помощников, тесты только для чтения, отмечайте найденные ошибки) и кнопка «Начать». Кнопка вызывает `POST start` и просит полноэкранный режим.
- [x] **Step 2:** Состояние `running`:
  - слева `FileTree`, файлы из `readOnly` с замком;
  - в центре Monaco `Editor` (язык `go`, `readOnly` для тестов); правки уходят по WS `{"type":"file"}` с задержкой 1 с после последнего ввода;
  - сверху `Timer`, который краснеет на последней минуте, и кнопки «Запустить тесты» и «Завершить»;
  - снизу `TestResults` из последнего прогона (имя, pass/fail/timeout, вывод по клику); во время прогона спиннер и выключенная кнопка;
  - кнопка «Здесь ошибка» берёт строку курсора и открывает поле для короткого комментария → `POST flags`;
  - список своих отметок.
- [x] **Step 3:** Состояние `finished`: `ReportView` (Task 12; до неё — заглушка «Сессия завершена»).
- [x] **Step 4:** Проверка: `npm run typecheck && npm run build`. Вручную на демо-проекте: начать, исправить одну мутацию, запустить тесты (число зелёных растёт), поставить отметку, обновить страницу (таймер продолжает с того же места).
- [ ] **Step 5:** Commit: `feat(web): экран студента`.

### Task 11: Экран аудитора

**Files:**
- Create: `web/src/pages/Auditor.tsx`, `web/src/components/MutationCard.tsx`

- [x] **Step 1:** Состояние `preparing`: этап (`baseline` → «Проверяем оригинал», `candidates` → «Ищем мутации», `triples` → «Проверяем независимость») и прогресс `done/total`. Состояние `failed`: текст ошибки и кнопка «Другой репозиторий».
- [x] **Step 2:** Состояния `ready` и `running`:
  - шапка: таймер, состояние, ссылка студента с кнопкой «Скопировать»;
  - три `MutationCard`: файл:строка, вид, `orig → repl`, текст подсказки (Task 14), статус «исправлена» по последнему прогону (все `killingTests[i]` прошли);
  - Monaco `DiffEditor`: слева `mutated[file]` (только чтение), справа текущий файл студента, обновляется по событию `file`. Строки мутаций слева подсвечены через `deltaDecorations`. Выбор файла — вкладками;
  - список отметок студента: у каждой кнопки «#1 / #2 / #3» и «Объяснение принято / Не принято» → `POST /api/a/{tok}/flags/{id}`;
  - история прогонов: время и число зелёных из общего числа;
  - кнопка «Завершить».
- [x] **Step 3:** Проверка: typecheck + build. Вручную в двух окнах (аудитор и студент в режиме инкогнито): правка студента видна аудитору за ≤ 2 с, отметка появляется у аудитора, привязка к #2 сохраняется после перезагрузки.
- [ ] **Step 4:** Commit: `feat(web): экран аудитора с живым diff и разбором отметок`.

### Task 12: Отчёт и сборка в один бинарник

**Files:**
- Create: `web/src/components/ReportView.tsx`
- Modify: `Makefile` (`build` зависит от `web`)

- [x] **Step 1:** `ReportView`: таблица по мутациям с колонками «Мутация» (файл:строка, `orig → repl`), «Обнаружена», «Объяснена», «Исправлена», «Время». Над таблицей плашка «Проект восстановлен / не восстановлен». Под ней блок «Сигналы» с подписью «события, не доказательства»: уходы со вкладки, вставки и их объём. Кнопка «Печать» (`window.print()`).
- [x] **Step 2:** Подключить `ReportView` у студента и у аудитора в состоянии `finished`.
- [x] **Step 3:** Run: `make build && ./bin/lcme` → `localhost:8080` отдаёт интерфейс без Vite. Глубокая ссылка `/a/<tok>` открывается после перезагрузки (SPA-фолбэк).
- [ ] **Step 4:** Commit: `feat(web): отчёт и встраивание интерфейса в бинарник`.

---

## Фаза 5. Публичная ссылка — контрольная точка MVP

### Task 13: Запуск на ноутбуке и сквозной прогон

**Files:**
- Create: `scripts/run.sh` (`set -a; . ./.env; set +a; exec ./bin/lcme`)
- Modify: `README.md` (реальные шаги, если что-то отличалось)

- [ ] **Step 1:** `.env`: `LCME_RUNNER=docker`, `LCME_PUBLIC_URL=https://<домен>.ngrok-free.app`.
- [ ] **Step 2:** Терминал 1: `make build && scripts/run.sh`. Терминал 2: `ngrok http 8080`. (Если Tailscale Funnel поднимется за 15 минут, используйте `tailscale funnel 8080` вместо ngrok: там нет страницы-предупреждения.)
- [ ] **Step 3:** С телефона по мобильному интернету открыть публичную ссылку и пройти сценарий целиком:
  1. Демо-проект → ready меньше чем за 60 с.
  2. Ссылка студента на втором устройстве → «Начать».
  3. Одна мутация исправлена → зелёных тестов стало больше.
  4. Отметка → аудитор привязал её к мутации и принял объяснение.
  5. «Завершить» → отчёт на обоих устройствах.
- [ ] **Step 4:** Тот же сценарий на реальном публичном Go-репозитории с тестами (подобрать заранее 1–2 штуки, например учебный проект из школы). Записать ссылки в `docs/demo.md`.
- [ ] **Step 5:** Перезапустить `scripts/run.sh` посреди сессии → сессия продолжилась, таймер не сбросился.
- [ ] **Step 6:** Commit: `chore: запуск на ноутбуке и проверка сквозного сценария`.

**MVP готов, когда:** все шаги 3–5 прошли по публичной ссылке, а `go test ./...` и `LCME_DOCKER=1 go test ./internal/sandbox/...` зелёные.

---

## Фаза 6. Желательное

### Task 14: Подсказки для аудитора (`internal/hints`)

**Files:**
- Create: `internal/hints/openrouter.go`
- Modify: `internal/hints/hints.go`, `cmd/server/main.go`
- Test: `internal/hints/hints_test.go`

**Interfaces:**
- Produces:
  ```go
  type Hint struct { MutationID, Explanation, Question, Source string } // Source: "llm" | "template"
  type Input struct { M mutate.Mutation; Snippet string } // ±3 строки оригинала вокруг мутации
  type Hinter interface { Hints(ctx context.Context, in []Input) ([]Hint, error) }
  func Template() Hinter
  func OpenRouter(apiKey, model, baseURL string, c *http.Client) Hinter // baseURL по умолчанию "https://openrouter.ai/api/v1"
  func Fallback(primary, secondary Hinter) Hinter
  ```

- [x] **Step 1: Тесты**

```go
func TestTemplateCoversAllKinds(t *testing.T)    // для каждого Kind: Explanation и Question не пусты, Source "template"
func TestOpenRouterRequest(t *testing.T)         // httptest: POST /chat/completions, заголовок "Authorization: Bearer k",
                                                 // в теле model == "m"; ответ choices[0].message.content =
                                                 // `[{"mutationId":"m1","explanation":"e","question":"q"}]` → Hint{m1,e,q,"llm"}
func TestOpenRouterBadJSONFallsBack(t *testing.T) // content "не JSON" → Fallback возвращает шаблонные подсказки без ошибки
func TestOpenRouterTimeout(t *testing.T)          // сервер спит 3 с, ctx 1 с → Fallback возвращает шаблоны быстрее 2 с
```

- [ ] **Step 2:** Run → FAIL. **Step 3:** Реализовать. Промпт на русском: роль «помощник аудитора». Для каждой мутации дать вид, `orig → repl` и сниппет; просить JSON-массив объектов `{mutationId, explanation, question}`. В объяснении: какую логику ломает мутация и какой навык проверяет. Вопрос — тот, что аудитор задаст студенту, без подсказки ответа. Из content извлекать JSON между первой `[` и последней `]`. `main.go`: если `OPENROUTER_API_KEY` и `LCME_LLM_MODEL` заданы, использовать `Fallback(OpenRouter(...), Template())`, иначе `Template()`.
- [ ] **Step 4:** Run → PASS. Вручную с реальным ключом: на демо-проекте у трёх карточек есть пометка «ИИ».
- [ ] **Step 5:** Commit: `feat(hints): подсказки аудитору через OpenRouter с шаблонным запасным вариантом`.

### Task 15: Сигналы, QR-код, доводка интерфейса

**Files:**
- Modify: `web/src/pages/Student.tsx`, `web/src/pages/Auditor.tsx`, `internal/audit/session.go`
- Test: `internal/audit/session_test.go`

- [x] **Step 1: Тест** `TestAddEventKinds`: `"hidden"`, `"visible"` и `"paste"` принимаются, `"keylog"` → ошибка; события после `Deadline` игнорируются.
- [x] **Step 2:** Run → FAIL → реализовать проверку в `AddEvent` → PASS.
- [x] **Step 3:** Студент: `document.visibilitychange` → событие `hidden`/`visible`; `editor.onDidPaste(e)` → событие `paste` с длиной вставленного текста. Аудитор: лента событий с подписью «сигналы, не доказательства» и QR-код ссылки студента (`qrcode.react`).
- [x] **Step 4:** Доводка: пустые и загрузочные состояния на всех экранах, понятные тексты ошибок, тёмная тема Monaco, адаптация под экран проектора 1280×720.
- [ ] **Step 5:** Проверка: typecheck + build + сквозной сценарий из Task 13 Step 3.
- [ ] **Step 6:** Commit: `feat: сигналы вкладки и вставок, QR-код, доводка интерфейса`.

---

## Фаза 7. Живой тест и заморозка (06.11 07:00–10:30)

### Task 16: Проверка на реальных людях

- [ ] **Step 1:** Один товарищ по команде проходит сессию студентом на своём реальном Go-проекте. Вы аудитор. Всё, что мешало, записать в `docs/demo.md`.
- [ ] **Step 2:** Исправлять только то, что ломает сценарий или вводит в заблуждение. Каждое исправление — новый тест или повтор шага из Task 13.
- [ ] **Step 3:** `go test ./... && LCME_DOCKER=1 go test ./internal/sandbox/...` → PASS.
- [ ] **Step 4:** **10:30 — заморозка кода.** После этого только тексты и README.
- [ ] **Step 5:** Commit + тег: `git tag v1.0-demo`.

## Фаза 8. Питч (10:30–13:00)

### Task 17: Презентация, видео, ответы

- [ ] **Step 1:** Слайды по структуре из Приложения 2 положения:
  - 0:00–0:30 — проблема: ИИ пишет проект, студент его не понимает;
  - 0:30–1:10 — идея: мутации, доказанные падающим тестом, независимость тройки по 8 комбинациям, отчёт «объяснил → исправил → прошёл»;
  - 1:10–2:20 — живое демо (сессия демо-проекта подготовлена заранее и находится в состоянии `ready`);
  - 2:20–2:50 — ценность для Tomorrow School и развитие (другие языки, интеграция с платформой аудитов);
  - 2:50–3:00 — итог.
- [ ] **Step 2:** Записать запасное видео сквозного сценария (1,5 мин) на случай проблем с сетью.
- [ ] **Step 3:** Шпаргалка ответов на вопросы:
  - **Эквивалентные мутанты:** в общем случае их не распознать, поэтому берём только мутации, на которых падает тест.
  - **Тестов нет:** честный отказ. Режим сравнения вывода — следующий шаг.
  - **Как запрещаете LLM:** запретить нельзя, делаем видимым; решение за аудитором.
  - **Чужой код:** контейнер без сети, без привилегий, uid 65534, лимиты, таймауты.
  - **Почему каждый тест отдельно:** паника роняет весь пакет; цифры 18/40 и 40/40.
  - **Какой ИИ и для чего** (п. 9.4): Claude при разработке, OpenRouter для подсказок аудитору.
- [ ] **Step 4:** 5 прогонов питча с таймером, каждый не больше 3:00.

## Фаза 9. Сдача (13:00–14:00)

### Task 18: Форма

- [ ] **Step 1:** Ноутбук на зарядке, сон отключён, `scripts/run.sh` и ngrok запущены. Публичная ссылка открывается с телефона.
- [ ] **Step 2:** В официальной Google-форме указать: название команды, «Кейс 3: Live Code Mutation Engine», ссылку на продукт (публичный домен), ссылку на презентацию (PDF), ссылку на GitHub.
- [ ] **Step 3:** **Отправить до 14:00.** Сделать скриншот подтверждения.
- [ ] **Step 4:** Взять с собой на площадку: зарядку, раздачу интернета с телефона, видео на флешке и в телефоне.
