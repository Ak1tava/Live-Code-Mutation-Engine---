# Контракт ядра Mutagen (для авторов вариантов)

Ядро `mutagen-core.js` хранит данные и поведение. **Вся разметка, компоновка и оформление — в варианте.** Ядро не менять: о проблемах сообщать ведущему агенту.

## Файлы варианта

```
design-opus-5/0N-name/index.html   подключает ../core/fixture.js, ../core/mutagen-core.js, style.css, app.js
                     /style.css    токены и стили
                     /app.js       MG.boot({ id:'0N-name', render })
```

`index.html`: `<html lang="ru">`, `<meta viewport>`, `<title>Mutagen — NN Name</title>`, `<div id="app"></div>`, скрипты обычные (`<script src>`, без `type=module`: страница открывается с `file://`). Никаких CDN и внешних шрифтов — только системные стеки с кириллицей.

## Рендер

`render(shell)` возвращает HTML-строку. Ядро вызывает её при каждом изменении и морфит DOM в `#app`, поэтому фокус, значения полей и прокрутка сохраняются. Обработчики не вешать: всё через атрибуты. Текст данных экранировать `MG.esc()`.

Высота: панель прототипа занимает верх страницы, `#app` — всё остальное и сам прокручивается. Рабочие экраны делайте `height:100%` с внутренней прокруткой панелей.

`render` ветвится по `shell.view`: `home | student | auditor | result`. Для `result` есть `shell.role` (`auditor | student`).

### Атрибуты

| Атрибут | Смысл |
|---|---|
| `data-act="name" data-arg="x" data-arg2="y"` | клик вызывает `MG.act.name(x, y)`; `disabled` уважается |
| `<form data-submit="submitUrl">` | отправка формы (Enter) вызывает действие |
| `<input data-bind="url" value="...">`, `data-bind="flagNote"` | двусторонняя привязка поля |
| `<select data-change="linkFlag" data-arg="f3">` | изменение вызывает `MG.act.linkFlag('f3', value)`; выбранный `<option selected>` указывайте сами |
| `data-mg-timer` | ядро само обновляет текст `MM:SS` и ставит `data-level="ok|warn|crit"` |
| `data-key="..."` | устойчивый ключ элемента в списках |
| `data-autofocus` | фокус при появлении |
| `data-mg-modal` | контейнер диалога: удержание Tab, Esc вызывает `dismiss`, возврат фокуса |
| `data-diff-line="N"` | строка diff с номером N в выданном коде: сюда прокручивается выбор мутации/отметки (в ближайшем прокручиваемом предке) |

### Редактор

```html
<div data-mg-editor data-file="textkit.go"></div>
```
Ядро монтирует настоящий редактор: номера строк, подсветка Go, правка в textarea, Tab, выбор строки по клику. Высота — от родителя (`100%`). Опционально: `data-line-action="1"` — кнопка «Здесь ошибка» у выбранной строки (класс `.mg-ed-lineact`, оформить в варианте; на мобильном скрыть); `data-margin="1"` — колонка заметок-отметок на полях (`.mg-ed-margin`, `.mg-ed-note`).

CSS-переменные редактора (задать в `:root` варианта): `--code-font --code-size --code-lh --code-bg --code-fg --code-caret --code-gutter-bg --code-gutter-fg --code-gutter-border --code-sel-line --code-selection --code-flag --code-chg --code-margin-w --ui-font --focus-ring`. Подсветка: `.tk-k` ключевые слова, `.tk-t` типы, `.tk-s` строки, `.tk-n` числа, `.tk-c` комментарии, `.tk-f` функции. Классы состояний: `.mg-ed-ln.is-sel`, `.mg-ed-line.is-sel`, `.is-flag`.

## Данные

- `MG.COPY` — общие тексты: `product, tagline, format, formatParts, steps[{title,text}], urlPlaceholder, prepareAction, demoAction, eventsCaption, rules[], prepFailed{title,reason,facts,next,retry}, unavailable{title,auditor,student}, reportPending`. Можно переформатировать, смысл и факты не менять.
- `MG.shell()` — `view, role, phase, preset, conn, connLabel, save, saveLabel, running, finishAsk, timerText, timerSec, timerLevel, timerLabel, resultReady, project{name,module}`.
  - `phase`: `preparing | prep-failed | ready | active | final-check | finished | unavailable`.
- `MG.home()` — `url, error, preparing, failed, prep{stage,total,stages[],current,demo}`.
- `MG.local` — локальное состояние варианта (вкладки, свёрнутые панели). Менять: `data-act="ui" data-arg="key" data-arg2="val"`, `data-act="uiToggle" data-arg="key"`.

### `MG.student()` — единственный источник данных экрана студента

`rules, format, files[{name,readOnly,isTest,active}], activeFile, activeIsReadOnly, selLine, canEdit, canRun, runReason, canFlag, flagReason, canFinish, canStart, draft{file,line,note,error}|null, flags[{id,file,line,note,at,atText}], runs[] (новые первыми), lastRun, runView, openTests{name:true}, summary` + всё из `shell()`.

`summary` (после завершения): `status (restored|not_restored|unavailable), statusLabel, durationText, message, reasonLabel`.

**В студенческом экране нельзя вызывать `MG.auditor()`.** Студент не видит мутаций, исходных значений, подсказок, привязок тестов, оценок. Не подсвечивайте строки 21/33/66 особо и не упоминайте слово «мутация» на этом экране (только «ошибки»).

### `MG.auditor()`

`link, copied, prep, mutations[], selected, flags[], unlinkedCount, pendingCount, canDecide, canFinish, events[], eventsCaption, files[], auditFile, diff(opts), report, runs, lastRun, runView, openTests` + `shell()`.

- Мутация: `id, n, title, func, file, line, kind, orig, repl, explanation, question, hintSource, killingTest, selected, detected, flagId, detectedLabel, explState (none|pending|accepted|rejected), explLabel, fixed, fixedAt, fixedAtText, fixedLabel`.
- Отметка: `id, file, line, note, atText, mutationId, mutationLabel, decision (true|false|null), decisionLabel`.
- `diff({context:3})` (или `context:Infinity`) → `{file, rows, added, removed, changed, leftTitle:'Выданный код', rightTitle:'Код студента'}`. Строка: `{kind:'row', type:'same|change|del|add', left:{n,text,html}|null, right:{n,text,html}|null, mut:{id,n,func,selected}|null}` или `{kind:'gap', count}`. `html` уже подсвечен и экранирован. «Выданный код» уже содержит мутации — это не исходная исправная версия; подпись не меняйте.
- `report`: `status, statusLabel, statusNote, durationText, reasonLabel, finalRun, mutations[+ fixedFinal, fixedFinalLabel, detectedFinalLabel, explFinalLabel], hiddenCount, pasteCount, pasteChars, events[], eventsCaption`. `fixedAtText` — время первого успешного исправления.

### Прогон (`run`)

`id, n, at, atText, final, kind, label, short, passed, total, tests[{name,status:'pass|fail|timeout',code:'PASS|FAIL|TIMEOUT',statusLabel,output}], message, output`.

`kind` обязателен к различению в интерфейсе — текст и иконка, не только цвет:

- `tests` — тесты выполнены (`status` различает падение и таймаут);
- `build` — ошибка сборки: показать `message` и `output`;
- `infra` — инфраструктурный сбой: показать `message`, не как неудачу студента;
- `none` — нет результата: показать `message`.

`lastRun === null` — тесты ещё не запускались; показать это отдельно.

## Действия (`data-act`)

| Группа | Действия |
|---|---|
| Навигация | `nav(view)` — обычно через панель прототипа; внутри продукта допустимо `nav('result')`, `nav('home')` |
| Главная | `submitUrl`, `tryDemo`, `prepRetry` |
| Студент | `start`, `selectFile(name)`, `run`, `toggleTest(name)`, `selectRun(id)`, `flagStart`, `flagCancel`, `flagSave`, `flagRemove(id)`, `selectFlag(id)`, `reconnect`, `retrySave` |
| Аудитор | `selectMutation(id)`, `selectAuditFile(name)`, `selectFlag(id)`, `linkFlag(flagId, mutationId|'')`, `accept(flagId)`, `reject(flagId)`, `clearDecision(flagId)`, `copyLink`, `toggleTest(name)`, `selectRun(id)` |
| Завершение | `finishAsk`, `finishCancel`, `finishConfirm`, `print` |

Автопроверка кликает именно по `data-act`, поэтому имена обязательны: кнопка «Запустить тесты» — `data-act="run"`, «Здесь ошибка» — `flagStart` в панели инструментов (видна всегда, также на телефоне), «Начать» — `start`, «Подготовить аудит» — submit `submitUrl`, поле — `data-bind="url"`, «Попробовать демо» — `tryDemo`, принять/отклонить — `accept`/`reject`, копирование — `copyLink`, завершение — `finishAsk` → `finishConfirm`, печать — `print`.

## Что должно быть в каждом состоянии

| view \ phase | preparing | prep-failed | ready | active + running/reconnecting/unsaved | final-check | finished / unavailable |
|---|---|---|---|---|---|---|
| home | этапы подготовки, блокировка формы | ошибка, причина, факты, `prepRetry` и `tryDemo` | форма | форма | форма | форма |
| student | «Проект ещё готовится» | «Аудит не начат» без технических деталей | правила + «Начать» | рабочее место | редактор только чтение, «Финальная проверка…» | сводка + ссылка на результат (`nav('result')`) |
| auditor | прогресс подготовки | причина + `nav('home')` | ссылка для студента, мутации, «Ожидает студента» | рабочее место | «Финальная проверка…» | рабочее место без завершения + `nav('result')` |
| result | `reportPending` | `reportPending` | `reportPending` | `reportPending` | `reportPending` | отчёт по роли |

Формулировки статусов: `connLabel`, `saveLabel`. При `unsaved` кнопка `retrySave`, при `reconnecting` кнопка `reconnect`. При `running` кнопка запуска заблокирована со спиннером и текстом. В диалоге завершения при `save==='unsaved'` предупреждайте о несохранённых правках. Не показывайте «Студент не справился», общий балл, вероятность списывания, оценку честности.

## Обязательные детали

- Файлы `isTest`: метка «Только чтение» в дереве и в заголовке редактора.
- Три результата мутации всегда раздельно: `detectedLabel`, `explLabel`, `fixedLabel`.
- Под событиями всегда подпись `eventsCaption`.
- Подсказка ИИ только у аудитора, с подписью `hintSource`.
- Мобильная ширина 390 px: вкладки или сворачивание, `body`/`#app` без горизонтальной прокрутки, код прокручивается внутри своей области.
- Токены в `:root` варианта: фон и поверхности, текст, акцент, границы, success/warning/danger, шрифты, интервалы, скругления, тени (если есть). Видимый `:focus-visible`, состояния hover/disabled/loading, `@media (prefers-reduced-motion: reduce)` уже обрабатывается ядром. `@media print` для отчёта.
