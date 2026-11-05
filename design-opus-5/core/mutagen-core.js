/* Mutagen — общее ядро прототипов DESIGN-1.
 * Здесь только демонстрационные данные и поведение. Компоновка и оформление — в каждом варианте.
 * Сервера нет: импорт, прогоны и таймер симулируются. API описан в core/CONTRACT.md. */
(function () {
  'use strict';

  /* ---------- Варианты ---------- */
  const VARIANTS = [
    { id: '01-clarity', n: '01', name: 'Clarity', sub: 'Светлое рабочее пространство', idea: 'Минимум визуальной нагрузки: код на весь экран, одна правая панель.' },
    { id: '02-workbench', n: '02', name: 'Workbench', sub: 'Консоль разработчика', idea: 'Привычная среда разработки: панели, дерево файлов, нижний док с тестами.' },
    { id: '03-folio', n: '03', name: 'Folio', sub: 'Пространство разбора', idea: 'Аудит как документ: код в основной колонке, заметки на полях у своих строк.' },
    { id: '04-focus', n: '04', name: 'Focus', sub: 'Последовательная работа', idea: 'Одна текущая задача в центре, остальное раскрывается по запросу.' },
    { id: '05-grid', n: '05', name: 'Grid', sub: 'Контрастная система', idea: 'Модульная сетка: три блока мутаций сверху, большой diff под ними.' },
  ];

  /* ---------- Демонстрационные данные (общие для всех вариантов) ---------- */
  const ISSUED = window.MG_FIXTURE || {};
  const FILES = ['textkit.go', 'textkit_test.go', 'go.mod'];
  const READ_ONLY = ['textkit_test.go'];
  const TESTS = ['TestMax', 'TestIsPrime', 'TestCapitalize', 'TestAtoiBase', 'TestWindowSums'];
  const SESSION_SEC = 600;
  const SNAPSHOT_REMAINING = 402; // 06:42

  const MUTATIONS = [
    {
      id: 'm1', n: 1, func: 'IsPrime', file: 'textkit.go', line: 21, kind: 'Граница цикла',
      orig: 'i*i <= n', repl: 'i*i < n', test: 'TestIsPrime',
      fix: ['i*i < n', 'i*i <= n'],
      explanation: 'Цикл останавливается раньше: делитель, равный корню из n, не проверяется. Поэтому квадраты простых чисел — 4, 9, 25 — считаются простыми.',
      question: 'Почему IsPrime(9) возвращает true, хотя 9 делится на 3? Какое значение i цикл не успевает проверить?',
    },
    {
      id: 'm2', n: 2, func: 'Capitalize', file: 'textkit.go', line: 33, kind: 'Граничное значение',
      orig: 'len(w) > 0', repl: 'len(w) > 1', test: 'TestCapitalize',
      fix: ['len(w) > 1', 'len(w) > 0'],
      explanation: 'Слова из одной буквы не попадают в условие и остаются без изменений: «a» не становится «A». Более длинные слова обрабатываются верно, поэтому ошибка видна только на коротком вводе.',
      question: 'Что произойдёт со словом из одной буквы? Почему остальные случаи теста при этом проходят?',
    },
    {
      id: 'm3', n: 3, func: 'WindowSums', file: 'textkit.go', line: 66, kind: 'Сдвиг на единицу',
      orig: 'i >= k', repl: 'i > k', test: 'TestWindowSums',
      fix: ['if i > k {', 'if i >= k {'],
      explanation: 'Элемент выходит из окна на шаг позже, поэтому сумма считается по расширенному окну: для [1 2 3 4] и k = 2 получается [3 6 8] вместо [3 5 7].',
      question: 'Какой элемент должен выйти из окна на шаге i = k? Что содержит sum в этот момент?',
    },
  ];

  const PASS_CHECK = {
    TestIsPrime: c => /i\s*\*\s*i\s*<=\s*n/.test(c),
    TestCapitalize: c => /len\(w\)\s*(>\s*0|>=\s*1|!=\s*0)/.test(c),
    TestWindowSums: c => /if\s+i\s*>=\s*k\s*\{/.test(c),
  };
  // Вывод реальный: получен запуском go test на этой же фикстуре.
  const FAIL_OUTPUT = {
    TestIsPrime: '--- FAIL: TestIsPrime (0.00s)\n    textkit_test.go:36: IsPrime(4) = true, want false\n    textkit_test.go:36: IsPrime(9) = true, want false\n    textkit_test.go:36: IsPrime(25) = true, want false',
    TestCapitalize: '--- FAIL: TestCapitalize (0.00s)\n    textkit_test.go:50: Capitalize("a") = "a", want "A"',
    TestWindowSums: '--- FAIL: TestWindowSums (0.00s)\n    textkit_test.go:87: WindowSums([1 2 3 4], 2) = [3 6 8], want [3 5 7]',
  };

  const COPY = {
    product: 'Mutagen',
    technical: 'Live Code Mutation Engine',
    tagline: 'Проверка понимания кода на взаимном аудите. Студент находит, объясняет и исправляет ошибки в собственном проекте.',
    format: '3 ошибки · 10 минут · Go',
    formatParts: ['3 ошибки', '10 минут', 'Go'],
    steps: [
      { key: 'prepare', title: 'Подготовка', text: 'Система проверяет проект и подбирает три ошибки, каждую подтверждает тестами.' },
      { key: 'code', title: 'Работа с кодом', text: 'Студент за 10 минут ищет ошибки, отмечает строки и исправляет код.' },
      { key: 'review', title: 'Разбор', text: 'Аудитор следит за изменениями, оценивает объяснения и получает отчёт.' },
    ],
    urlPlaceholder: 'https://github.com/владелец/репозиторий',
    prepareAction: 'Подготовить аудит',
    demoAction: 'Попробовать демо',
    eventsCaption: 'События, не доказательства',
    rules: [
      'В проекте 3 логические ошибки. На поиск, объяснение и исправление — 10 минут.',
      'Файлы *_test.go доступны только для чтения.',
      'Выберите строку и нажмите «Здесь ошибка», чтобы оставить короткое объяснение.',
      'ИИ-помощника здесь нет. После «Начать» таймер идёт на сервере и не останавливается при перезагрузке.',
    ],
    prepFailed: {
      title: 'Не удалось подобрать три мутации',
      reason: 'Тесты проекта покрывают мало логики: система не смогла подтвердить три независимые ошибки.',
      facts: 'Кандидатов: 11 · подтверждены тестами: 2 · нужно независимых: 3',
      next: 'Добавьте тесты на логику проекта и запустите подготовку снова или выберите другой репозиторий.',
      retry: 'Изменить ссылку',
    },
    unavailable: {
      title: 'Оценка недоступна',
      auditor: 'Финальная проверка завершилась техническим сбоем. Это не результат студента: восстановление проекта подтвердить нельзя.',
      student: 'Финальная проверка завершилась техническим сбоем. Это не оценка вашей работы — аудитор сообщит о дальнейших шагах.',
    },
    reportPending: 'Отчёт появится после завершения сессии.',
  };

  const PREP_STAGES = [
    'Проверяем репозиторий и go.mod',
    'Запускаем исходные тесты',
    'Проверяем кандидатов в мутации',
    'Подтверждаем независимую тройку',
  ];

  const PRESETS = [
    { key: 'preparing', label: 'Подготовка' },
    { key: 'ready', label: 'Готовность к старту' },
    { key: 'active', label: 'Активная сессия' },
    { key: 'running', label: 'Выполнение тестов' },
    { key: 'reconnecting', label: 'Переподключение' },
    { key: 'unsaved', label: 'Несохранённые изменения' },
    { key: 'final-check', label: 'Финальная проверка' },
    { key: 'finished', label: 'Завершение' },
    { key: 'prep-failed', label: 'Ошибка подготовки' },
    { key: 'unavailable', label: 'Недоступная оценка' },
  ];
  const OUTCOMES = [
    { key: 'normal', label: 'Обычный (по коду)' },
    { key: 'timeout', label: 'Таймаут теста' },
    { key: 'build', label: 'Ошибка сборки' },
    { key: 'infra', label: 'Инфраструктурный сбой' },
    { key: 'none', label: 'Нет результата' },
  ];

  /* ---------- Утилиты ---------- */
  const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const pad = n => String(n).padStart(2, '0');
  const time = sec => { const s = Math.max(0, Math.round(sec)); return pad(Math.floor(s / 60)) + ':' + pad(s % 60); };
  const plural = (n, one, few, many) => { const a = Math.abs(n) % 100, b = a % 10; return a > 10 && a < 20 ? many : b > 1 && b < 5 ? few : b === 1 ? one : many; };
  const clone = o => JSON.parse(JSON.stringify(o));

  const GO_RE = /(\/\/.*$)|("(?:\\.|[^"\\])*"|`[^`]*`|'(?:\\.|[^'\\])*')|\b(func|return|if|else|for|range|var|const|type|struct|package|import|map|switch|case|default|break|continue|go|defer|nil|true|false|interface|chan|select)\b|\b(int|int64|string|bool|rune|byte|error|float64|uint)\b|\b(\d+)\b|\b([A-Za-z_]\w*)(?=\()/g;
  function highlightGo(src) {
    let out = '', last = 0, m;
    GO_RE.lastIndex = 0;
    while ((m = GO_RE.exec(src))) {
      out += esc(src.slice(last, m.index));
      const cls = m[1] ? 'c' : m[2] ? 's' : m[3] ? 'k' : m[4] ? 't' : m[5] ? 'n' : 'f';
      out += '<span class="tk-' + cls + '">' + esc(m[0]) + '</span>';
      last = m.index + m[0].length;
      if (m[0].length === 0) GO_RE.lastIndex++;
    }
    return out + esc(src.slice(last));
  }

  function lcsDiff(aLines, bLines) {
    const n = aLines.length, m = bLines.length;
    const dp = Array.from({ length: n + 1 }, () => new Uint16Array(m + 1));
    for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--)
      dp[i][j] = aLines[i] === bLines[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    const ops = []; let i = 0, j = 0;
    while (i < n || j < m) {
      if (i < n && j < m && aLines[i] === bLines[j]) { ops.push({ t: 'same', a: i + 1, b: j + 1 }); i++; j++; }
      else if (j < m && (i === n || dp[i][j + 1] > dp[i + 1][j])) { ops.push({ t: 'add', b: j + 1 }); j++; }
      else { ops.push({ t: 'del', a: i + 1 }); i++; }
    }
    return ops;
  }
  function changedHtml(text, other) {
    let p = 0; while (p < text.length && p < other.length && text[p] === other[p]) p++;
    let s = 0; while (s < text.length - p && s < other.length - p && text[text.length - 1 - s] === other[other.length - 1 - s]) s++;
    const mid = text.slice(p, text.length - s);
    if (!mid) return highlightGo(text);
    return highlightGo(text.slice(0, p)) + '<mark class="mg-chg">' + esc(mid) + '</mark>' + highlightGo(text.slice(text.length - s));
  }
  /* rows: {kind:'row', type:'same'|'change'|'del'|'add', left:{n,text,html}|null, right:{n,text,html}|null, mut:{id,n,func,selected}|null}
   *     | {kind:'gap', count} */
  function buildDiff(file, opts) {
    opts = opts || {};
    const a = (ISSUED[file] || '').split('\n'), b = (S.code[file] || '').split('\n');
    if (a[a.length - 1] === '') a.pop();
    if (b[b.length - 1] === '') b.pop();
    const ops = lcsDiff(a, b), rows = [];
    let added = 0, removed = 0;
    for (let k = 0; k < ops.length; k++) {
      const op = ops[k];
      if (op.t === 'same') {
        const h = highlightGo(a[op.a - 1]);
        rows.push({ kind: 'row', type: 'same', left: { n: op.a, text: a[op.a - 1], html: h }, right: { n: op.b, text: b[op.b - 1], html: h } });
      } else if (op.t === 'del') {
        const next = ops[k + 1]; removed++;
        if (next && next.t === 'add') {
          added++; k++;
          rows.push({ kind: 'row', type: 'change', left: { n: op.a, text: a[op.a - 1], html: changedHtml(a[op.a - 1], b[next.b - 1]) }, right: { n: next.b, text: b[next.b - 1], html: changedHtml(b[next.b - 1], a[op.a - 1]) } });
        } else rows.push({ kind: 'row', type: 'del', left: { n: op.a, text: a[op.a - 1], html: highlightGo(a[op.a - 1]) }, right: null });
      } else {
        added++;
        rows.push({ kind: 'row', type: 'add', left: null, right: { n: op.b, text: b[op.b - 1], html: highlightGo(b[op.b - 1]) } });
      }
    }
    const muts = MUTATIONS.filter(m => m.file === file);
    rows.forEach(r => {
      const m = muts.find(x => r.left && r.left.n === x.line);
      r.mut = m ? { id: m.id, n: m.n, func: m.func, selected: S.selMutation === m.id } : null;
    });
    const context = opts.context == null ? 3 : opts.context;
    const keep = rows.map(() => context === Infinity);
    rows.forEach((r, i) => {
      if (r.type !== 'same' || r.mut) for (let k = Math.max(0, i - context); k <= Math.min(rows.length - 1, i + context); k++) keep[k] = true;
    });
    const out = []; let gap = 0;
    rows.forEach((r, i) => {
      if (keep[i]) { if (gap) { out.push({ kind: 'gap', count: gap }); gap = 0; } out.push(r); } else gap++;
    });
    if (gap) out.push({ kind: 'gap', count: gap });
    return { file, rows: out, added, removed, changed: added + removed > 0, leftTitle: 'Выданный код', rightTitle: 'Код студента' };
  }

  /* ---------- Прогоны ---------- */
  function evaluate(code) {
    const src = code['textkit.go'] || '';
    const bare = src.replace(/\/\/.*$/gm, '').replace(/"(?:\\.|[^"\\])*"/g, '""');
    let depth = 0, paren = 0, bad = false, lineNo = 1, errLine = 0;
    for (const ch of bare) {
      if (ch === '\n') lineNo++;
      if (ch === '{') depth++; if (ch === '}') depth--;
      if (ch === '(') paren++; if (ch === ')') paren--;
      if ((depth < 0 || paren < 0) && !bad) { bad = true; errLine = lineNo; }
    }
    if (depth !== 0 || paren !== 0 || bad) return { build: { line: errLine || lineNo } };
    return { tests: TESTS.map(name => ({ name, pass: PASS_CHECK[name] ? PASS_CHECK[name](src) : true })) };
  }
  function makeRun(n, at, code, outcome, final) {
    const base = { id: 'r' + n, n, at, atText: time(at), final: !!final, tests: [], passed: 0, total: TESTS.length, message: null, output: null };
    const ev = evaluate(code);
    if (outcome === 'infra') return Object.assign(base, { kind: 'infra', label: 'Инфраструктурный сбой', short: 'сбой', message: 'Песочница не ответила. Это технический сбой, а не результат работы над кодом. Тесты не выполнены.' });
    if (outcome === 'none') return Object.assign(base, { kind: 'none', label: 'Нет результата', short: 'нет данных', message: 'Проверка не вернула результат. Считать его успехом или ошибкой нельзя.' });
    if (outcome === 'build' || ev.build) {
      const line = ev.build ? ev.build.line : 34;
      return Object.assign(base, { kind: 'build', label: 'Ошибка сборки', short: 'сборка', message: 'Код не компилируется, поэтому тесты не запускались.', output: '# example.com/textkit\n./textkit.go:' + line + ':2: syntax error: unexpected token, expected expression' });
    }
    const tests = ev.tests.map(t => {
      const timedOut = outcome === 'timeout' && t.name === 'TestWindowSums';
      const status = timedOut ? 'timeout' : t.pass ? 'pass' : 'fail';
      return {
        name: t.name, status,
        code: status === 'pass' ? 'PASS' : status === 'fail' ? 'FAIL' : 'TIMEOUT',
        statusLabel: status === 'pass' ? 'Проходит' : status === 'fail' ? 'Падает' : 'Таймаут',
        output: timedOut ? 'panic: test timed out after 2s\nrunning tests:\n\tTestWindowSums (2s)' : t.pass ? '--- PASS: ' + t.name + ' (0.00s)' : FAIL_OUTPUT[t.name],
      };
    });
    const passed = tests.filter(t => t.status === 'pass').length, timeouts = tests.filter(t => t.status === 'timeout').length;
    return Object.assign(base, {
      kind: 'tests', tests, passed,
      label: passed + ' из ' + tests.length + ' ' + plural(tests.length, 'теста', 'тестов', 'тестов') + ' проходят' + (timeouts ? ' · ' + timeouts + ' таймаут' : ''),
      short: passed + '/' + tests.length,
    });
  }

  /* ---------- Состояние ---------- */
  function issuedCode() { return { 'textkit.go': ISSUED['textkit.go'], 'textkit_test.go': ISSUED['textkit_test.go'], 'go.mod': ISSUED['go.mod'] }; }
  function freshSession() {
    return {
      phase: 'ready', code: issuedCode(), runs: [], flags: [], events: [], selMutation: 'm1', activeFile: 'textkit.go', selLine: null, auditFile: 'textkit.go',
      conn: 'online', save: 'saved', running: false, hold: false, finishAsk: false, draft: null, openTests: {}, runViewId: null, copied: false,
      timer: { paused: true, remaining: SESSION_SEC, deadline: null }, durationSec: null, finishReason: null, prep: null, nextFlag: 1,
    };
  }
  function snapshotSession() {
    const s = freshSession();
    s.phase = 'active';
    const c0 = issuedCode(), c1 = issuedCode();
    c1['textkit.go'] = c1['textkit.go'].replace(MUTATIONS[0].fix[0], MUTATIONS[0].fix[1]);
    s.code = c1;
    s.runs = [makeRun(1, 41, c0, 'normal'), makeRun(2, 134, c1, 'normal'), makeRun(3, 185, c1, 'normal')];
    s.flags = [
      { id: 'f1', file: 'textkit.go', line: 21, at: 91, note: 'Цикл не доходит до корня: для 9 делитель 3 не проверяется, поэтому 9 считается простым.', mutationId: 'm1', decision: true },
      { id: 'f2', file: 'textkit.go', line: 33, at: 171, note: 'Слово из одной буквы не попадает в условие и остаётся строчным.', mutationId: 'm2', decision: null },
      { id: 'f3', file: 'textkit.go', line: 52, at: 184, note: 'Проверить порядок вычисления: похоже, основание умножается не там.', mutationId: null, decision: null },
    ];
    s.events = [
      { id: 'e1', at: 66, kind: 'hidden', label: 'Уход со вкладки', detail: '14 с', size: 14 },
      { id: 'e2', at: 152, kind: 'paste', label: 'Вставка из буфера', detail: '12 символов', size: 12 },
    ];
    s.nextFlag = 4;
    s.selMutation = 'm2';
    s.selLine = 33;
    s.openTests = { TestCapitalize: true };
    s.timer = { paused: false, remaining: SNAPSHOT_REMAINING, deadline: Date.now() + SNAPSHOT_REMAINING * 1000 };
    return s;
  }

  let VARIANT = null, renderFn = null, appEl = null, frozen = false, embed = false, persist = true, flowToken = 0, pendingScroll = null, pendingDiffLine = null;
  let S = Object.assign(snapshotSession(), { view: 'home', role: 'auditor', outcome: 'normal', home: { url: '', error: null } });
  const local = {};

  function remaining() {
    const t = S.timer;
    if (t.paused || !t.deadline) return t.remaining;
    return Math.max(0, Math.ceil((t.deadline - Date.now()) / 1000));
  }
  function pauseTimer() { S.timer.remaining = remaining(); S.timer.paused = true; S.timer.deadline = null; }
  function resumeTimer() { if (frozen) return; S.timer.paused = false; S.timer.deadline = Date.now() + S.timer.remaining * 1000; }
  const elapsed = () => SESSION_SEC - remaining();
  function timerLevel(sec) { return sec <= 60 ? 'crit' : sec <= 120 ? 'warn' : 'ok'; }

  function currentPreset() {
    const p = S.phase;
    if (p === 'active') {
      if (S.running) return 'running';
      if (S.conn === 'reconnecting') return 'reconnecting';
      if (S.save === 'unsaved') return 'unsaved';
      return 'active';
    }
    return p;
  }

  /* ---------- Представления: что разрешено видеть роли ---------- */
  function shell() {
    const sec = remaining();
    return {
      view: S.view, role: S.role, phase: S.phase, preset: currentPreset(), presetLabel: (PRESETS.find(p => p.key === currentPreset()) || {}).label || '',
      conn: S.conn, connLabel: S.conn === 'online' ? 'Соединение есть' : 'Переподключение…',
      save: S.save, saveLabel: S.save === 'saved' ? 'Сохранено' : S.save === 'saving' ? 'Сохраняем…' : 'Есть несохранённые изменения',
      running: S.running, finishAsk: S.finishAsk,
      timerText: time(sec), timerSec: sec, timerLevel: timerLevel(sec),
      timerLabel: S.phase === 'ready' ? 'Не запущен' : S.phase === 'active' ? 'Осталось' : 'Остановлен',
      resultReady: S.phase === 'finished' || S.phase === 'unavailable',
      project: { name: 'textkit', module: 'example.com/textkit', isExample: true },
    };
  }
  function viewFiles() {
    return FILES.map(name => ({ name, readOnly: READ_ONLY.includes(name), isTest: /_test\.go$/.test(name), active: S.activeFile === name }));
  }
  function publicRun(r) { return r; }
  function runSelection() {
    const last = S.runs[S.runs.length - 1] || null;
    const view = (S.runViewId && S.runs.find(r => r.id === S.runViewId)) || last;
    return { runs: S.runs.map(publicRun).slice().reverse(), lastRun: last, runView: view, openTests: Object.assign({}, S.openTests) };
  }

  /* student(): единственный источник данных для экрана студента. Мутаций, подсказок, привязок и оценок здесь нет. */
  function student() {
    const sh = shell();
    const active = S.phase === 'active';
    const ro = READ_ONLY.includes(S.activeFile);
    const runReason = !active ? 'Сессия не активна' : S.running ? 'Тесты выполняются' : S.conn !== 'online' ? 'Нет соединения с сервером' : null;
    const flagReason = !active ? 'Сессия не активна' : ro ? 'Отмечать можно только файлы проекта' : !S.selLine ? 'Сначала выберите строку' : null;
    const summary = sh.resultReady ? {
      status: studentStatus(),
      statusLabel: studentStatus() === 'restored' ? 'Проект восстановлен' : studentStatus() === 'not_restored' ? 'Проект не восстановлен' : COPY.unavailable.title,
      durationSec: S.durationSec, durationText: time(S.durationSec || 0),
      message: studentStatus() === 'unavailable' ? COPY.unavailable.student : 'Сессия завершена. Подробности разберёт аудитор.',
      reasonLabel: S.finishReason === 'timeout' ? 'Время истекло' : 'Завершено досрочно',
    } : null;
    return Object.assign({}, sh, runSelection(), {
      rules: COPY.rules, format: COPY.format,
      files: viewFiles(), activeFile: S.activeFile, activeIsReadOnly: ro,
      selLine: S.selLine,
      canEdit: active && !ro,
      canRun: !runReason, runReason,
      canFlag: !flagReason, flagReason,
      canFinish: active,
      canStart: S.phase === 'ready',
      draft: S.draft ? { file: S.draft.file, line: S.draft.line, note: S.draft.note, error: S.draft.error } : null,
      flags: S.flags.map(f => ({ id: f.id, file: f.file, line: f.line, note: f.note, at: f.at, atText: time(f.at) })),
      summary,
    });
  }
  function studentStatus() { return reportStatus(); }

  function reportStatus() {
    if (S.phase === 'unavailable') return 'unavailable';
    const fin = S.runs[S.runs.length - 1];
    if (!fin) return 'unavailable';
    if (fin.kind === 'infra' || fin.kind === 'none') return 'unavailable';
    return fin.kind === 'tests' && fin.passed === fin.total ? 'restored' : 'not_restored';
  }
  function testStatusIn(run, name) { const t = run.tests.find(x => x.name === name); return t ? t.status : null; }
  function firstFixed(m) { const r = S.runs.find(x => x.kind === 'tests' && testStatusIn(x, m.test) === 'pass'); return r ? r.at : null; }
  function lastTestRun() { for (let i = S.runs.length - 1; i >= 0; i--) if (S.runs[i].kind === 'tests') return S.runs[i]; return null; }
  function mutationView(m) {
    const flag = S.flags.find(f => f.mutationId === m.id && f.decision === true) || S.flags.find(f => f.mutationId === m.id && f.decision === null) || S.flags.find(f => f.mutationId === m.id) || null;
    const expl = !flag ? 'none' : flag.decision === true ? 'accepted' : flag.decision === false ? 'rejected' : 'pending';
    const lt = lastTestRun();
    const fixed = !!lt && testStatusIn(lt, m.test) === 'pass';
    const first = firstFixed(m);
    return {
      id: m.id, n: m.n, title: 'Мутация ' + m.n, func: m.func, file: m.file, line: m.line, kind: m.kind, orig: m.orig, repl: m.repl,
      explanation: m.explanation, question: m.question, hintSource: 'Подсказка ИИ для аудитора', killingTest: m.test,
      selected: S.selMutation === m.id,
      detected: !!flag, flagId: flag ? flag.id : null,
      detectedLabel: flag ? 'Обнаружена' : 'Не обнаружена',
      explState: expl,
      explLabel: { none: 'Нет объяснения', pending: 'Ожидает оценки', accepted: 'Объяснение принято', rejected: 'Объяснение отклонено' }[expl],
      fixed, fixedAt: first, fixedAtText: first == null ? null : time(first),
      fixedLabel: fixed ? 'Исправлена' + (first != null ? ' на ' + time(first) : '') : 'Не исправлена',
    };
  }
  function auditor() {
    const sh = shell();
    const mutations = MUTATIONS.map(mutationView);
    const flags = S.flags.map(f => {
      const m = MUTATIONS.find(x => x.id === f.mutationId);
      return {
        id: f.id, file: f.file, line: f.line, note: f.note, at: f.at, atText: time(f.at), mutationId: f.mutationId, mutationLabel: m ? 'Мутация ' + m.n + ' · ' + m.func : 'Не привязана',
        decision: f.decision, decisionLabel: f.decision === true ? 'Объяснение принято' : f.decision === false ? 'Объяснение отклонено' : 'Ожидает оценки',
      };
    });
    const canDecide = S.phase === 'active' || S.phase === 'final-check' || S.phase === 'finished' || S.phase === 'unavailable';
    const events = S.events.map(e => Object.assign({}, e, { atText: time(e.at) }));
    return Object.assign({}, sh, runSelection(), {
      link: 'https://mutagen.example/s/k7f2-9qm4', copied: S.copied,
      prep: S.prep ? { stage: S.prep.stage, total: PREP_STAGES.length, stages: PREP_STAGES, current: PREP_STAGES[S.prep.stage] || '', demo: S.prep.demo } : null,
      mutations, selected: mutations.find(m => m.selected) || mutations[0],
      flags, unlinkedCount: flags.filter(f => !f.mutationId).length, pendingCount: flags.filter(f => f.mutationId && f.decision === null).length,
      canDecide, canFinish: S.phase === 'active',
      events, eventsCaption: COPY.eventsCaption,
      files: FILES.map(name => ({ name, readOnly: READ_ONLY.includes(name), isTest: /_test\.go$/.test(name), active: S.auditFile === name })),
      auditFile: S.auditFile,
      diff: opts => buildDiff(S.auditFile, opts),
      report: sh.resultReady ? report() : null,
    });
  }
  function report() {
    const status = reportStatus();
    const mutations = MUTATIONS.map(mutationView).map(m => Object.assign(m, {
      fixedFinal: status === 'unavailable' ? null : m.fixed,
      fixedFinalLabel: status === 'unavailable' ? 'Нет данных' : m.fixed ? 'Да' : 'Нет',
      detectedFinalLabel: m.detected ? 'Да' : 'Нет',
      explFinalLabel: { none: '—', pending: 'Не оценено', accepted: 'Принято', rejected: 'Отклонено' }[m.explState],
    }));
    const fin = S.runs[S.runs.length - 1] || null;
    return {
      status,
      statusLabel: status === 'restored' ? 'Проект восстановлен' : status === 'not_restored' ? 'Проект не восстановлен' : COPY.unavailable.title,
      statusNote: status === 'unavailable' ? COPY.unavailable.auditor : status === 'restored' ? 'Все тесты проходят в финальной проверке.' : (fin && fin.kind === 'build' ? 'Код не компилируется: тесты не запускались.' : (fin ? fin.label + ' в финальной проверке.' : '')),
      durationSec: S.durationSec, durationText: time(S.durationSec || 0),
      reasonLabel: S.finishReason === 'timeout' ? 'Время истекло' : 'Завершено досрочно',
      finalRun: fin, mutations,
      flags: S.flags.length,
      hiddenCount: S.events.filter(e => e.kind === 'hidden').length,
      pasteCount: S.events.filter(e => e.kind === 'paste').length,
      pasteChars: S.events.filter(e => e.kind === 'paste').reduce((a, e) => a + (e.size || 0), 0),
      events: S.events.map(e => Object.assign({}, e, { atText: time(e.at) })),
      eventsCaption: COPY.eventsCaption,
    };
  }

  /* ---------- Действия ---------- */
  function announce(text) { const el = document.getElementById('mg-live'); if (el) { el.textContent = ''; setTimeout(() => { el.textContent = text; }, 30); } }
  function validateUrl(raw) {
    const v = (raw || '').trim();
    if (!v) return 'Вставьте ссылку на публичный GitHub-репозиторий.';
    if (!/^https:\/\//i.test(v)) return 'Ссылка должна начинаться с https://github.com/';
    if (!/^https:\/\/github\.com\//i.test(v)) return 'Поддерживается только GitHub: https://github.com/владелец/репозиторий';
    if (!/^https:\/\/github\.com\/[\w.-]+\/[\w.-]+?(\.git)?\/?$/i.test(v)) return 'Нужна ссылка на репозиторий целиком: https://github.com/владелец/репозиторий';
    return null;
  }
  function loadSession(base) {
    const keep = { view: S.view, role: S.role, outcome: S.outcome, home: S.home };
    S = Object.assign(base, keep);
  }
  function startPrep(url) {
    flowToken++;
    const token = flowToken;
    loadSession(freshSession());
    S.phase = 'preparing'; S.view = 'home'; S.role = 'auditor';
    S.prep = { stage: 0, demo: !url, hold: false };
    commit();
    const step = () => {
      if (token !== flowToken || S.phase !== 'preparing') return;
      if (S.prep.stage < PREP_STAGES.length - 1) { S.prep.stage++; commit(); setTimeout(step, 650); return; }
      loadSession(freshSession());
      S.phase = 'ready'; S.view = 'auditor'; S.role = 'auditor';
      announce('Аудит подготовлен. Передайте ссылку студенту.');
      commit();
    };
    setTimeout(step, 650);
  }
  function doFinal(reason) {
    pauseTimer();
    S.finishAsk = false; S.draft = null; S.running = false;
    S.finishReason = reason;
    S.durationSec = SESSION_SEC - S.timer.remaining;
    const run = makeRun(S.runs.length + 1, S.durationSec, S.code, S.outcome, true);
    S.runs.push(run); S.runViewId = null;
    S.phase = run.kind === 'infra' || run.kind === 'none' ? 'unavailable' : 'finished';
    S.conn = 'online'; S.save = 'saved'; S.hold = false;
  }
  function startFinal(reason) {
    flowToken++;
    const token = flowToken;
    pauseTimer();
    S.phase = 'final-check'; S.finishAsk = false; S.draft = null; S.running = false; S.hold = false;
    commit();
    setTimeout(() => {
      if (token !== flowToken || S.phase !== 'final-check') return;
      doFinal(reason); S.view = 'result';
      announce('Сессия завершена.');
      commit();
    }, 1600);
  }
  let saveTimer = null;
  function scheduleSave() {
    clearTimeout(saveTimer);
    if (S.conn !== 'online') { S.save = 'unsaved'; return; }
    S.save = 'saving';
    saveTimer = setTimeout(() => { if (S.save === 'saving') { S.save = 'saved'; commit(); } }, 500);
  }

  const act = {
    /* навигация */
    nav(view) {
      if (view === 'home') S.role = 'auditor';
      if (view === 'auditor') S.role = 'auditor';
      if (view === 'student') S.role = 'student';
      S.view = view; commit();
    },
    setRole(role) {
      S.role = role;
      if (role === 'student' && (S.view === 'auditor' || S.view === 'home')) S.view = 'student';
      if (role === 'auditor' && S.view === 'student') S.view = 'auditor';
      commit();
    },
    /* главная */
    submitUrl() {
      const err = validateUrl(S.home.url);
      S.home.error = err;
      if (err) { announce(err); commit(); return; }
      startPrep(S.home.url.trim());
    },
    tryDemo() { S.home.error = null; startPrep(null); },
    prepRetry() { loadSession(snapshotSession()); S.view = 'home'; S.role = 'auditor'; S.home.error = null; commit(); },
    /* студент */
    start() { if (S.phase !== 'ready') return; S.phase = 'active'; S.timer.remaining = SESSION_SEC; resumeTimer(); announce('Сессия началась. Осталось 10 минут.'); commit(); },
    selectFile(name) { S.activeFile = name; S.selLine = null; S.draft = null; commit(); },
    selectLine(n) { const v = Number(n); if (S.selLine === v) return; S.selLine = v; commit(); },
    edit(file, text) {
      if (S.phase !== 'active' || READ_ONLY.includes(file)) return;
      S.code[file] = text; scheduleSave(); commit();
    },
    run() {
      if (S.phase !== 'active' || S.running || S.conn !== 'online') return;
      flowToken++;
      const token = flowToken;
      S.running = true; S.hold = false;
      S.save = 'saved';
      commit();
      setTimeout(() => {
        if (token !== flowToken || !S.running) return;
        S.running = false;
        const run = makeRun(S.runs.length + 1, elapsed(), S.code, S.outcome);
        S.runs.push(run); S.runViewId = null;
        if (run.kind === 'tests') run.tests.forEach(t => { if (t.status !== 'pass') S.openTests[t.name] = true; else delete S.openTests[t.name]; });
        announce('Прогон завершён: ' + run.label);
        commit();
      }, 1400);
    },
    toggleTest(name) { if (S.openTests[name]) delete S.openTests[name]; else S.openTests[name] = true; commit(); },
    selectRun(id) { S.runViewId = id || null; commit(); },
    flagStart() {
      if (S.phase !== 'active' || READ_ONLY.includes(S.activeFile) || !S.selLine) return;
      S.draft = { file: S.activeFile, line: S.selLine, note: '', error: null }; commit();
    },
    flagCancel() { S.draft = null; commit(); },
    flagSave() {
      if (!S.draft) return;
      if (!S.draft.note.trim()) { S.draft.error = 'Напишите короткое объяснение: что здесь не так.'; commit(); return; }
      const id = 'f' + S.nextFlag++;
      S.flags.push({ id, file: S.draft.file, line: S.draft.line, at: elapsed(), note: S.draft.note.trim(), mutationId: null, decision: null });
      announce('Отметка добавлена: строка ' + S.draft.line);
      S.draft = null; commit();
    },
    flagRemove(id) { S.flags = S.flags.filter(f => f.id !== id); commit(); },
    selectFlag(id) {
      const f = S.flags.find(x => x.id === id); if (!f) return;
      if (S.view === 'student') { S.activeFile = f.file; S.selLine = f.line; pendingScroll = { file: f.file, line: f.line }; }
      else { S.auditFile = f.file; if (f.mutationId) S.selMutation = f.mutationId; pendingDiffLine = f.line; S.selectedFlag = id; }
      commit();
    },
    reconnect() {
      setTimeout(() => { S.conn = 'online'; S.save = 'saved'; announce('Соединение восстановлено, изменения сохранены.'); commit(); }, 800);
    },
    retrySave() { S.save = 'saving'; commit(); setTimeout(() => { S.save = 'saved'; commit(); }, 600); },
    /* аудитор */
    selectMutation(id) { S.selMutation = id; const m = MUTATIONS.find(x => x.id === id); if (m) { S.auditFile = m.file; pendingDiffLine = m.line; } commit(); },
    selectAuditFile(name) { S.auditFile = name; commit(); },
    linkFlag(flagId, mutationId) {
      const f = S.flags.find(x => x.id === flagId); if (!f) return;
      f.mutationId = mutationId || null;
      if (!f.mutationId) f.decision = null;
      if (f.mutationId) { S.selMutation = f.mutationId; pendingDiffLine = f.line; }
      commit();
    },
    accept(flagId) { const f = S.flags.find(x => x.id === flagId); if (f && f.mutationId) { f.decision = true; announce('Объяснение принято.'); commit(); } },
    reject(flagId) { const f = S.flags.find(x => x.id === flagId); if (f && f.mutationId) { f.decision = false; announce('Объяснение отклонено.'); commit(); } },
    clearDecision(flagId) { const f = S.flags.find(x => x.id === flagId); if (f) { f.decision = null; commit(); } },
    copyLink() {
      const link = 'https://mutagen.example/s/k7f2-9qm4';
      const done = () => { S.copied = true; commit(); announce('Ссылка скопирована.'); setTimeout(() => { S.copied = false; commit(); }, 1800); };
      const fallback = () => { const t = document.createElement('textarea'); t.value = link; t.style.position = 'fixed'; t.style.opacity = '0'; document.body.appendChild(t); t.select(); try { document.execCommand('copy'); } catch (e) { /* демо */ } t.remove(); done(); };
      if (navigator.clipboard && window.isSecureContext) navigator.clipboard.writeText(link).then(done, fallback); else fallback();
    },
    /* завершение */
    finishAsk() { if (S.phase === 'active') { S.finishAsk = true; commit(); } },
    finishCancel() { S.finishAsk = false; commit(); },
    finishConfirm() { startFinal('manual'); },
    print() { window.print(); },
    /* панель прототипа */
    preset,
    outcome(k) { S.outcome = k; commit(); },
    demoFix(which) {
      if (S.phase !== 'active') return;
      MUTATIONS.filter(m => which === 'all' || m.id === which).forEach(m => { S.code[m.file] = S.code[m.file].replace(m.fix[0], m.fix[1]); });
      scheduleSave(); commit();
    },
    toggleTimer() { if (S.timer.paused) { frozen = false; resumeTimer(); } else { frozen = true; pauseTimer(); } commit(); },
    reset() { flowToken++; const v = S.view, r = S.role; S = Object.assign(snapshotSession(), { view: v, role: r, outcome: 'normal', home: { url: '', error: null } }); if (frozen) pauseTimer(); commit(); },
    /* локальное состояние варианта: вкладки, сворачивание */
    ui(key, val) { local[key] = val; commit(); },
    uiToggle(key) { local[key] = !local[key]; commit(); },
    dismiss() { if (S.finishAsk) act.finishCancel(); else if (S.draft) act.flagCancel(); },
  };

  function preset(name) {
    flowToken++;
    const sessionStates = ['active', 'running', 'reconnecting', 'unsaved', 'final-check', 'finished', 'unavailable'];
    const liveOrPost = S.phase === 'active' || S.phase === 'final-check';
    if (sessionStates.includes(name) && !liveOrPost) loadSession(snapshotSession());
    if (S.phase === 'final-check') { S.phase = 'active'; if (!frozen) resumeTimer(); }
    if (frozen) pauseTimer();
    S.finishAsk = false; S.running = false; S.hold = false; S.conn = 'online'; S.save = 'saved';
    if (name === 'running') { S.running = true; S.hold = true; }
    if (name === 'reconnecting') { S.conn = 'reconnecting'; S.save = 'unsaved'; }
    if (name === 'unsaved') { S.save = 'unsaved'; }
    if (name === 'final-check') { pauseTimer(); S.phase = 'final-check'; S.hold = true; }
    if (name === 'finished') { const o = S.outcome; S.outcome = (o === 'infra' || o === 'none') ? 'normal' : o; doFinal('manual'); S.outcome = o; }
    if (name === 'unavailable') { const o = S.outcome; S.outcome = 'infra'; doFinal('manual'); S.outcome = o; }
    if (name === 'ready') { loadSession(freshSession()); }
    if (name === 'preparing') { loadSession(freshSession()); S.phase = 'preparing'; S.prep = { stage: 2, demo: false, hold: true }; S.hold = true; if (S.role === 'auditor') S.view = 'home'; }
    if (name === 'prep-failed') { loadSession(freshSession()); S.phase = 'prep-failed'; S.prep = null; if (S.role === 'auditor') S.view = 'home'; }
    commit();
  }

  /* ---------- Морфинг DOM: сохраняет фокус, значения и прокрутку ---------- */
  function syncAttrs(from, to) {
    const keepOpen = from.tagName === 'DETAILS';
    for (const a of Array.from(from.attributes)) {
      if (keepOpen && a.name === 'open') continue;
      if (!to.hasAttribute(a.name)) from.removeAttribute(a.name);
    }
    for (const a of Array.from(to.attributes)) if (from.getAttribute(a.name) !== a.value) from.setAttribute(a.name, a.value);
    if (from.tagName === 'INPUT' || from.tagName === 'TEXTAREA') {
      if (to.hasAttribute('value') && from !== document.activeElement && from.value !== to.getAttribute('value')) from.value = to.getAttribute('value');
      if (from.type === 'checkbox') from.checked = to.hasAttribute('checked');
    }
    if (from.tagName === 'OPTION') from.selected = to.hasAttribute('selected');
  }
  function sameNode(a, b) {
    if (a.nodeType !== b.nodeType) return false;
    if (a.nodeType !== 1) return true;
    if (a.tagName !== b.tagName) return false;
    const ka = a.getAttribute('data-key'), kb = b.getAttribute('data-key');
    return ka === kb;
  }
  function morphNode(from, to) {
    if (from.nodeType !== 1) { if (from.nodeValue !== to.nodeValue) from.nodeValue = to.nodeValue; return; }
    syncAttrs(from, to);
    if (from.hasAttribute('data-mg-editor')) return; // редактор управляет своим содержимым
    if (from.tagName === 'TEXTAREA') return;
    morphChildren(from, to);
  }
  function morphChildren(parent, target) {
    const keyed = {};
    Array.from(parent.childNodes).forEach(c => { if (c.nodeType === 1 && c.hasAttribute('data-key')) keyed[c.getAttribute('data-key')] = c; });
    let cur = parent.firstChild;
    for (const next of Array.from(target.childNodes)) {
      const key = next.nodeType === 1 ? next.getAttribute('data-key') : null;
      if (key && keyed[key] && keyed[key].tagName === next.tagName) {
        const el = keyed[key]; delete keyed[key];
        if (el !== cur) parent.insertBefore(el, cur); else cur = cur.nextSibling;
        morphNode(el, next);
      } else if (cur && sameNode(cur, next)) {
        const el = cur; cur = cur.nextSibling;
        morphNode(el, next);
      } else {
        parent.insertBefore(next.cloneNode(true), cur);
      }
    }
    while (cur) { const n = cur.nextSibling; parent.removeChild(cur); cur = n; }
  }

  /* ---------- Редактор кода ---------- */
  function mountEditor(host) {
    host.innerHTML = '<div class="mg-ed"><div class="mg-ed-gutter" aria-hidden="true"></div><div class="mg-ed-main"><pre class="mg-ed-hl" aria-hidden="true"></pre>' +
      '<textarea class="mg-ed-ta" wrap="off" spellcheck="false" autocapitalize="off" autocomplete="off" autocorrect="off"></textarea><button type="button" class="mg-ed-lineact" data-act="flagStart" hidden></button></div><div class="mg-ed-margin" hidden></div></div>';
    const ed = { root: host.firstChild, gutter: host.querySelector('.mg-ed-gutter'), hl: host.querySelector('.mg-ed-hl'), ta: host.querySelector('textarea'), action: host.querySelector('.mg-ed-lineact'), margin: host.querySelector('.mg-ed-margin'), file: null, text: null };
    host._mg = ed;
    const pick = () => {
      const pos = ed.ta.selectionStart || 0;
      const n = ed.ta.value.slice(0, pos).split('\n').length;
      act.selectLine(n);
    };
    ed.ta.addEventListener('click', pick);
    ed.ta.addEventListener('keyup', pick);
    ed.ta.addEventListener('input', () => { renderLines(ed, ed.ta.value); ed.text = ed.ta.value; act.edit(ed.file, ed.ta.value); });
    ed.ta.addEventListener('paste', e => {
      if (S.phase !== 'active' || READ_ONLY.includes(ed.file)) return;
      const size = ((e.clipboardData && e.clipboardData.getData('text')) || '').length;
      S.events.push({ id: 'e' + (S.events.length + 1) + '-' + elapsed(), at: elapsed(), kind: 'paste', label: 'Вставка из буфера', detail: size + ' ' + plural(size, 'символ', 'символа', 'символов'), size });
      commit();
    });
    ed.ta.addEventListener('keydown', e => {
      if (e.key === 'Tab' && !ed.ta.readOnly && !ed.tabRelease) {
        e.preventDefault();
        const a = ed.ta.selectionStart, b = ed.ta.selectionEnd;
        ed.ta.setRangeText('\t', a, b, 'end');
        ed.ta.dispatchEvent(new Event('input', { bubbles: true }));
      }
      ed.tabRelease = e.key === 'Escape';
    });
    ed.gutter.addEventListener('click', e => { const t = e.target.closest('[data-n]'); if (t) act.selectLine(t.getAttribute('data-n')); });
    return ed;
  }
  function renderLines(ed, text) {
    const lines = text.split('\n');
    let g = '', h = '', w = 0;
    lines.forEach((l, i) => {
      const n = i + 1;
      g += '<div class="mg-ed-ln" data-n="' + n + '">' + n + '</div>';
      h += '<div class="mg-ed-line" data-n="' + n + '">' + (highlightGo(l) || ' ') + '</div>';
      const len = l.replace(/\t/g, '    ').length; if (len > w) w = len;
    });
    ed.gutter.innerHTML = g; ed.hl.innerHTML = h;
    ed.ta.style.minWidth = 'calc(' + (w + 4) + 'ch + var(--mg-ed-pad, 16px) * 2)';
    ed.ta.rows = lines.length;
  }
  function syncEditors() {
    appEl.querySelectorAll('[data-mg-editor]').forEach(host => {
      const ed = host._mg || mountEditor(host);
      const file = host.getAttribute('data-file') || S.activeFile;
      const text = S.code[file] || '';
      const ro = host.getAttribute('data-readonly') === '1' || READ_ONLY.includes(file) || S.phase !== 'active';
      if (ed.file !== file) { ed.file = file; ed.text = null; host.firstChild.scrollTop = 0; host.firstChild.scrollLeft = 0; }
      if (ed.text !== text) {
        ed.text = text;
        if (ed.ta.value !== text) ed.ta.value = text;
        renderLines(ed, text);
      }
      ed.ta.readOnly = ro;
      ed.ta.setAttribute('aria-label', 'Редактор кода: ' + file + (ro ? ', только чтение' : ''));
      host.classList.toggle('is-readonly', ro);
      const flags = S.flags.filter(f => f.file === file);
      ed.gutter.querySelectorAll('.is-sel,.is-flag').forEach(n => n.classList.remove('is-sel', 'is-flag'));
      ed.hl.querySelectorAll('.is-sel,.is-flag').forEach(n => n.classList.remove('is-sel', 'is-flag'));
      flags.forEach(f => {
        const a = ed.gutter.querySelector('[data-n="' + f.line + '"]'), b = ed.hl.querySelector('[data-n="' + f.line + '"]');
        if (a) a.classList.add('is-flag'); if (b) b.classList.add('is-flag');
      });
      if (S.selLine) {
        const a = ed.gutter.querySelector('[data-n="' + S.selLine + '"]'), b = ed.hl.querySelector('[data-n="' + S.selLine + '"]');
        if (a) a.classList.add('is-sel'); if (b) b.classList.add('is-sel');
      }
      const lh = lineHeight(ed);
      if (host.getAttribute('data-line-action') === '1' && S.selLine && S.phase === 'active' && !READ_ONLY.includes(file) && !S.draft) {
        ed.action.hidden = false;
        ed.action.textContent = host.getAttribute('data-line-action-label') || 'Здесь ошибка';
        ed.action.style.top = (S.selLine - 1) * lh + 'px';
      } else ed.action.hidden = true;
      if (host.getAttribute('data-margin') === '1') {
        ed.margin.hidden = false;
        ed.margin.style.height = ed.hl.offsetHeight + 'px';
        ed.margin.innerHTML = flags.map(f => '<button type="button" class="mg-ed-note" data-act="selectFlag" data-arg="' + f.id + '" style="top:' + (f.line - 1) * lh + 'px"><b>Строка ' + f.line + '</b> ' + esc(f.note) + '</button>').join('');
      } else ed.margin.hidden = true;
      if (pendingScroll && pendingScroll.file === file) {
        const sc = host.firstChild;
        sc.scrollTop = Math.max(0, (pendingScroll.line - 1) * lh - sc.clientHeight / 2);
        pendingScroll = null;
      }
    });
  }
  function lineHeight(ed) { const l = ed.hl.querySelector('.mg-ed-line'); return l ? l.getBoundingClientRect().height || 20 : 20; }

  function nearestScroller(el) {
    let p = el.parentElement;
    while (p && p !== document.body) {
      const o = getComputedStyle(p).overflowY;
      if ((o === 'auto' || o === 'scroll') && p.scrollHeight > p.clientHeight) return p;
      p = p.parentElement;
    }
    return null;
  }

  /* ---------- Рендер ---------- */
  let scheduled = false, lastModal = null, modalReturn = null;
  function commit() {
    if (scheduled) return;
    scheduled = true;
    (window.requestAnimationFrame || setTimeout)(flush);
    setTimeout(flush, 80); // фоновая вкладка и iframe вне экрана
  }
  function flush() {
    if (!scheduled) return;
    scheduled = false;
    if (!renderFn) return;
    const t = document.createElement('template');
    t.innerHTML = renderFn(shell());
    morphChildren(appEl, t.content);
    document.documentElement.setAttribute('data-mg-view', S.view);
    document.documentElement.setAttribute('data-mg-role', S.role);
    document.documentElement.setAttribute('data-mg-state', currentPreset());
    syncEditors();
    updateTimers();
    appEl.querySelectorAll('[data-autofocus]').forEach(el => { if (!el._mgFocused) { el._mgFocused = true; el.focus({ preventScroll: true }); } });
    const modal = appEl.querySelector('[data-mg-modal]');
    if (modal && modal !== lastModal) {
      if (!lastModal) modalReturn = document.activeElement;
      const f = modal.querySelector('[data-autofocus]') || modal.querySelector('button,input,select,textarea,a[href]');
      if (f) f.focus();
    } else if (!modal && lastModal && modalReturn && modalReturn.isConnected) { modalReturn.focus(); modalReturn = null; }
    lastModal = modal;
    if (pendingDiffLine != null) {
      const row = appEl.querySelector('[data-diff-line="' + pendingDiffLine + '"]');
      if (row) {
        const sc = nearestScroller(row);
        if (sc) { const r = row.getBoundingClientRect(), c = sc.getBoundingClientRect(); sc.scrollTop += r.top - c.top - c.height / 2 + r.height / 2; }
      }
      pendingDiffLine = null;
    }
    savePersist();
    if (panel) panel.sync();
  }
  function updateTimers() {
    const sec = remaining(), text = time(sec), lvl = timerLevel(sec);
    appEl.querySelectorAll('[data-mg-timer]').forEach(el => {
      if (el.textContent !== text) el.textContent = text;
      el.setAttribute('data-level', S.phase === 'active' ? lvl : 'ok');
    });
  }
  function tick() {
    if (S.phase === 'active' && !S.timer.paused && remaining() <= 0) { startFinal('timeout'); return; }
    updateTimers();
    if (panel) panel.sync();
  }

  /* ---------- Сохранение между переключениями и перезагрузкой ---------- */
  function storageKey() { return 'mg:' + VARIANT.id; }
  function savePersist() {
    if (!persist) return;
    try { sessionStorage.setItem(storageKey(), JSON.stringify(S)); } catch (e) { /* файл открыт без хранилища */ }
  }
  function loadPersist() {
    if (!persist) return;
    try {
      const raw = sessionStorage.getItem(storageKey());
      if (!raw) return;
      const s = JSON.parse(raw);
      if (!s || !s.code || !s.timer) return;
      S = s;
      if (S.running && !S.hold) S.running = false;
      if (S.phase === 'preparing' && !(S.prep && S.prep.hold)) { const v = S.view, r = S.role, o = S.outcome, h = S.home; S = Object.assign(freshSession(), { view: v, role: r, outcome: o, home: h }); }
      if (S.phase === 'final-check' && !S.hold) { doFinal(S.finishReason || 'manual'); S.view = 'result'; }
    } catch (e) { /* игнорируем */ }
  }

  /* ---------- Панель просмотра прототипа (отделена от продукта) ---------- */
  let panel = null;
  const PANEL_CSS = `
    :host{all:initial;display:block;position:relative;z-index:50;font:13px/1.3 system-ui,"Segoe UI",sans-serif;color:#f3f0e6}
    *{box-sizing:border-box}
    .bar{display:flex;align-items:center;gap:10px;min-height:40px;padding:4px 10px;background:#2b2a26;background-image:repeating-linear-gradient(135deg,rgba(255,255,255,.035) 0 6px,transparent 6px 12px);border-bottom:2px solid #e8b931}
    .tag{font-weight:700;letter-spacing:.08em;font-size:11px;background:#e8b931;color:#2b2a26;padding:3px 6px;border-radius:3px;white-space:nowrap}
    .note{color:#cfcabb;font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;min-width:0;flex:0 1 auto}
    .grp{display:flex;align-items:center;gap:6px;white-space:nowrap}
    .grp>span{color:#b5b0a0;font-size:12px}
    .spacer{flex:1 1 0;min-width:0}
    button,select,a.btn{font:inherit;color:#f3f0e6;background:#3b3933;border:1px solid #66625a;border-radius:4px;height:28px;padding:0 8px;cursor:pointer;text-decoration:none;display:inline-flex;align-items:center}
    select{padding-right:2px;max-width:190px}
    option{color:#111;background:#fff}
    button:hover,a.btn:hover,select:hover{background:#4a4740}
    button[aria-pressed=true]{background:#e8b931;color:#2b2a26;border-color:#e8b931;font-weight:600}
    button:disabled{opacity:.5;cursor:not-allowed}
    :focus-visible{outline:2px solid #fff;outline-offset:1px}
    .seg{display:inline-flex}.seg button{border-radius:0;margin-left:-1px}.seg button:first-child{border-radius:4px 0 0 4px;margin-left:0}.seg button:last-child{border-radius:0 4px 4px 0}
    .more{position:absolute;right:8px;top:100%;margin-top:4px;width:320px;background:#2b2a26;border:1px solid #66625a;border-top:2px solid #e8b931;border-radius:4px;padding:10px;display:grid;gap:10px;box-shadow:0 8px 20px rgba(0,0,0,.35)}
    .more[hidden]{display:none}
    .more label,.more .row{display:grid;gap:4px;color:#cfcabb;font-size:12px}
    .more select{max-width:none;width:100%}
    .btns{display:flex;flex-wrap:wrap;gap:6px}
    .hint{color:#b5b0a0;font-size:12px}
    .mob{display:none}
    @media (max-width:1180px){.note{display:none}}
    @media (max-width:820px){
      .bar{flex-wrap:wrap}
      .desk{display:none!important}
      .mob{display:inline-flex}
      .sheet-extra{display:grid!important}
      .more{left:8px;right:8px;width:auto;max-height:70vh;overflow:auto}
    }
    .sheet-extra{display:none;gap:10px}
    @media (prefers-reduced-motion:no-preference){button{transition:background .12s}}
  `;
  function viewOf() { return S.view; }
  function buildPanel() {
    const host = document.createElement('div');
    host.id = 'mg-preview';
    host.setAttribute('aria-label', 'Панель просмотра прототипа');
    host.setAttribute('role', 'region');
    document.body.insertBefore(host, document.body.firstChild);
    const root = host.attachShadow({ mode: 'open' });
    const views = [['home', 'Главная'], ['student', 'Студент'], ['auditor', 'Аудитор'], ['result', 'Результат']];
    const here = location.pathname.replace(/\\/g, '/');
    root.innerHTML = '<style>' + PANEL_CSS + '</style>' +
      '<div class="bar">' +
      '<span class="tag">ПРОТОТИП</span>' +
      '<span class="note">Локальные данные: импорт, тесты и таймер симулируются.</span>' +
      '<a class="btn desk" href="../index.html">Галерея</a>' +
      '<label class="grp desk"><span>Вариант</span><select id="variant" aria-label="Вариант">' + VARIANTS.map(v => '<option value="' + v.id + '">' + v.n + ' ' + v.name + '</option>').join('') + '</select></label>' +
      '<div class="spacer desk"></div>' +
      '<div class="grp desk"><span>Экран</span><div class="seg" id="views">' + views.map(v => '<button type="button" data-v="' + v[0] + '">' + v[1] + '</button>').join('') + '</div></div>' +
      '<div class="grp desk"><span>Роль</span><div class="seg" id="roles"><button type="button" data-r="auditor">Аудитор</button><button type="button" data-r="student">Студент</button></div></div>' +
      '<label class="grp desk"><span>Состояние</span><select id="preset" aria-label="Состояние">' + PRESETS.map(p => '<option value="' + p.key + '">' + p.label + '</option>').join('') + '</select></label>' +
      '<select class="mob" id="mview" aria-label="Экран">' + views.map(v => '<option value="' + v[0] + '">' + v[1] + '</option>').join('') + '</select>' +
      '<div class="spacer mob"></div>' +
      '<button type="button" id="moreBtn" aria-expanded="false">Ещё</button>' +
      '</div>' +
      '<div class="more" id="more" hidden>' +
      '<div class="sheet-extra"><span class="hint">Локальные данные: импорт, тесты и таймер симулируются.</span>' +
      '<div class="row">Роль<div class="seg" id="roles2"><button type="button" data-r="auditor">Аудитор</button><button type="button" data-r="student">Студент</button></div></div>' +
      '<label>Состояние<select id="preset2">' + PRESETS.map(p => '<option value="' + p.key + '">' + p.label + '</option>').join('') + '</select></label>' +
      '<label>Вариант<select id="variant2">' + VARIANTS.map(v => '<option value="' + v.id + '">' + v.n + ' ' + v.name + '</option>').join('') + '</select></label>' +
      '<a class="btn" href="../index.html">Галерея</a></div>' +
      '<label>Исход следующего прогона<select id="outcome">' + OUTCOMES.map(o => '<option value="' + o.key + '">' + o.label + '</option>').join('') + '</select></label>' +
      '<div class="row">Правка за студента (строки выданного кода)<div class="btns"><button type="button" id="fix2">Исправить строку 33</button><button type="button" id="fixAll">Исправить все</button></div></div>' +
      '<div class="row">Прогон<div class="btns"><button type="button" id="runBtn">Запустить демо-прогон</button></div></div>' +
      '<div class="row">Таймер<div class="btns"><button type="button" id="timerBtn">Пауза</button><button type="button" id="resetBtn">Вернуть снимок 06:42</button></div></div>' +
      '</div>';
    const $ = id => root.getElementById(id);
    root.querySelectorAll('[data-v]').forEach(b => b.addEventListener('click', () => act.nav(b.dataset.v)));
    root.querySelectorAll('[data-r]').forEach(b => b.addEventListener('click', () => act.setRole(b.dataset.r)));
    $('mview').addEventListener('change', e => act.nav(e.target.value));
    [$('preset'), $('preset2')].forEach(s => s.addEventListener('change', e => act.preset(e.target.value)));
    [$('variant'), $('variant2')].forEach(s => s.addEventListener('change', e => {
      location.href = '../' + e.target.value + '/index.html?view=' + S.view + '&role=' + S.role + '&state=' + currentPreset();
    }));
    $('outcome').addEventListener('change', e => act.outcome(e.target.value));
    $('fix2').addEventListener('click', () => act.demoFix('m2'));
    $('fixAll').addEventListener('click', () => act.demoFix('all'));
    $('runBtn').addEventListener('click', () => act.run());
    $('timerBtn').addEventListener('click', () => act.toggleTimer());
    $('resetBtn').addEventListener('click', () => act.preset('active') || act.reset());
    $('moreBtn').addEventListener('click', () => { const m = $('more'); m.hidden = !m.hidden; $('moreBtn').setAttribute('aria-expanded', String(!m.hidden)); });
    document.addEventListener('click', e => { if (!host.contains(e.target)) { $('more').hidden = true; $('moreBtn').setAttribute('aria-expanded', 'false'); } });
    return {
      sync() {
        const p = currentPreset();
        root.querySelectorAll('[data-v]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.v === S.view)));
        root.querySelectorAll('[data-r]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.r === S.role)));
        ['preset', 'preset2'].forEach(i => { if ($(i).value !== p) $(i).value = p; });
        ['variant', 'variant2'].forEach(i => { if ($(i).value !== VARIANT.id) $(i).value = VARIANT.id; });
        if ($('mview').value !== S.view) $('mview').value = S.view;
        if ($('outcome').value !== S.outcome) $('outcome').value = S.outcome;
        const live = S.phase === 'active';
        $('fix2').disabled = !live; $('fixAll').disabled = !live; $('runBtn').disabled = !live || S.running || S.conn !== 'online';
        $('timerBtn').textContent = S.timer.paused ? 'Запустить' : 'Пауза';
        $('timerBtn').disabled = !live;
      },
    };
  }

  /* ---------- Базовые стили: только структура редактора и служебные классы ---------- */
  const BASE_CSS = `
    html,body{margin:0;height:100%}
    body{display:flex;flex-direction:column;min-height:0}
    #app{flex:1 1 auto;min-height:0;min-width:0;overflow:auto;position:relative}
    .mg-sr{position:absolute!important;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap}
    [data-mg-editor]{display:block;min-height:0;min-width:0;height:100%;overflow:hidden;background:var(--code-bg,#fff);color:var(--code-fg,#111)}
    .mg-ed{height:100%;overflow:auto;display:grid;grid-template-columns:auto minmax(max-content,1fr) auto;align-items:start;font-family:var(--code-font,Consolas,monospace);font-size:var(--code-size,14px);line-height:var(--code-lh,22px);tab-size:4;-moz-tab-size:4;--mg-ed-pad:16px;overscroll-behavior:contain}
    .mg-ed-gutter{position:sticky;left:0;z-index:3;background:var(--code-gutter-bg,var(--code-bg,#fff));color:var(--code-gutter-fg,#777);text-align:right;user-select:none;min-height:100%;border-right:1px solid var(--code-gutter-border,transparent)}
    .mg-ed-ln{padding:0 12px 0 14px;min-width:3.2em;height:var(--code-lh,22px);cursor:pointer;position:relative}
    .mg-ed-ln:hover{color:var(--code-fg,#111)}
    .mg-ed-ln.is-sel{color:var(--code-fg,#111);font-weight:600;background:var(--code-sel-line,rgba(0,0,0,.05))}
    .mg-ed-ln.is-flag::before{content:"⚑";position:absolute;left:3px;color:var(--code-flag,#b45309);font-weight:400}
    .mg-ed-main{position:relative;min-width:max-content;min-height:100%}
    .mg-ed-hl{margin:0;padding:0;font:inherit;line-height:inherit;tab-size:inherit;white-space:pre}
    .mg-ed-line{padding:0 var(--mg-ed-pad);height:var(--code-lh,22px)}
    .mg-ed-line.is-sel{background:var(--code-sel-line,rgba(0,0,0,.05))}
    .mg-ed-line.is-flag{box-shadow:inset 2px 0 0 var(--code-flag,#b45309)}
    .mg-ed-ta{position:absolute;inset:0;width:100%;height:100%;margin:0;border:0;outline:0;resize:none;overflow:hidden;background:transparent;color:transparent;caret-color:var(--code-caret,#111);padding:0 var(--mg-ed-pad);font:inherit;line-height:inherit;tab-size:inherit;white-space:pre;display:block}
    .mg-ed-ta::selection{background:var(--code-selection,rgba(50,100,220,.28));color:transparent}
    .mg-ed-ta:focus-visible{outline:none}
    [data-mg-editor]:focus-within{box-shadow:inset 0 0 0 2px var(--focus-ring,transparent)}
    .mg-ed-lineact{position:absolute;right:8px;z-index:2;height:var(--code-lh,22px);line-height:1;font:600 12px/1 inherit;font-family:var(--ui-font,system-ui);cursor:pointer}
    .mg-ed-lineact[hidden]{display:none}
    .mg-ed-margin{position:sticky;right:0;width:var(--code-margin-w,280px);z-index:2}
    .mg-ed-margin[hidden]{display:none}
    .mg-ed-note{position:absolute;left:0;right:0;text-align:left;font-family:var(--ui-font,system-ui);font-size:13px;line-height:1.35;cursor:pointer}
    .mg-chg{background:var(--code-chg,rgba(255,200,0,.35));color:inherit;border-radius:2px}
    @media print{#mg-preview{display:none!important}#app{overflow:visible}html,body{height:auto;display:block}}
    @media (prefers-reduced-motion:reduce){*,*::before,*::after{animation-duration:.001ms!important;animation-iteration-count:1!important;transition-duration:.001ms!important;scroll-behavior:auto!important}}
  `;

  /* ---------- Делегирование событий ---------- */
  function bindEvents() {
    document.addEventListener('click', e => {
      const t = e.target.closest && e.target.closest('[data-act]');
      if (!t || t.disabled || t.getAttribute('aria-disabled') === 'true') return;
      if (!appEl.contains(t)) return;
      const fn = act[t.getAttribute('data-act')];
      if (!fn) { console.error('Mutagen: неизвестное действие', t.getAttribute('data-act')); return; }
      if (t.tagName === 'A') e.preventDefault();
      fn(t.getAttribute('data-arg'), t.getAttribute('data-arg2'));
    });
    document.addEventListener('submit', e => {
      const f = e.target.closest('[data-submit]');
      if (!f) return;
      e.preventDefault();
      const fn = act[f.getAttribute('data-submit')];
      if (fn) fn();
    });
    document.addEventListener('input', e => {
      const b = e.target.getAttribute && e.target.getAttribute('data-bind');
      if (!b) return;
      if (b === 'url') { S.home.url = e.target.value; S.home.error = null; }
      if (b === 'flagNote' && S.draft) { S.draft.note = e.target.value; S.draft.error = null; }
      commit();
    });
    document.addEventListener('change', e => {
      const t = e.target;
      const name = t.getAttribute && t.getAttribute('data-change');
      if (!name || !act[name]) return;
      const a = t.getAttribute('data-arg');
      if (a != null) act[name](a, t.value); else act[name](t.value);
    });
    document.addEventListener('keydown', e => {
      const modal = appEl.querySelector('[data-mg-modal]');
      if (e.key === 'Escape' && (modal || S.draft)) { e.preventDefault(); act.dismiss(); return; }
      if (e.key === 'Tab' && modal) {
        const items = Array.from(modal.querySelectorAll('button:not([disabled]),input:not([disabled]),select,textarea,a[href]')).filter(x => x.offsetParent !== null);
        if (!items.length) return;
        const first = items[0], last = items[items.length - 1];
        if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
        else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
      }
    });
  }

  /* ---------- Запуск ---------- */
  function boot(cfg) {
    VARIANT = VARIANTS.find(v => v.id === cfg.id) || { id: cfg.id, n: '', name: cfg.id };
    renderFn = cfg.render;
    const q = new URLSearchParams(location.search);
    embed = q.get('embed') === '1';
    frozen = q.get('frozen') === '1';
    if (embed || frozen) persist = false;
    const style = document.createElement('style');
    style.textContent = BASE_CSS;
    document.head.insertBefore(style, document.head.firstChild);
    appEl = document.getElementById('app');
    if (!appEl) { appEl = document.createElement('div'); appEl.id = 'app'; document.body.appendChild(appEl); }
    const live = document.createElement('div');
    live.id = 'mg-live'; live.className = 'mg-sr'; live.setAttribute('aria-live', 'polite');
    document.body.appendChild(live);
    if (embed) document.documentElement.classList.add('mg-embed');
    loadPersist();
    if (frozen) pauseTimer();
    bindEvents();
    if (!embed) panel = buildPanel();
    const role = q.get('role'), view = q.get('view'), st = q.get('state');
    if (role === 'auditor' || role === 'student') S.role = role;
    if (view) { S.view = view; if (view === 'student') S.role = 'student'; if (view === 'auditor' || view === 'home') S.role = 'auditor'; }
    if (st && PRESETS.some(p => p.key === st)) preset(st);
    commit();
    flush();
    setInterval(tick, 500);
    if (frozen) pauseTimer();
    window.addEventListener('beforeunload', savePersist);
  }

  window.MG = {
    VARIANTS, PRESETS, OUTCOMES, COPY, FILES,
    esc, time, plural, highlightGo,
    boot, shell, student, auditor, act, local,
    home: () => ({ url: S.home.url, error: S.home.error, preparing: S.phase === 'preparing', failed: S.phase === 'prep-failed', prep: S.prep ? { stage: S.prep.stage, total: PREP_STAGES.length, stages: PREP_STAGES, current: PREP_STAGES[S.prep.stage] || '', demo: S.prep.demo } : null }),
    /** Служебное: для автоматической проверки. В вариантах не использовать. */
    _debug: () => S,
  };
})();
