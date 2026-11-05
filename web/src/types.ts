export type SessionState = 'preparing' | 'ready' | 'running' | 'finished' | 'failed'

export interface TestResult {
  pkg: string
  name: string
  status: 'pass' | 'fail' | 'timeout'
  panicked: boolean
  output: string
}

export interface RunView {
  at: string
  final: boolean
  result: { id: string; buildError: string; infrastructureError?: string; tests: TestResult[] | null; durationMs: number }
}

export interface Flag {
  id: string
  file: string
  line: number
  note: string
}

export interface AuditorFlag extends Flag {
  at: string
  mutationId: string
  accepted: boolean | null
}

export interface ReportSummary {
  restored: boolean
  durationSec: number
  verificationError?: string
}

export interface Mutation {
  id: string
  file: string
  line: number
  kind: string
  orig: string
  repl: string
}

export interface MutationReport extends Mutation {
  detected: boolean
  explained: boolean
  fixed: boolean
  fixedAfterSec: number
}

export interface Report extends ReportSummary {
  mutations: MutationReport[]
  hiddenCount: number
  pasteCount: number
  pasteChars: number
}

export interface StudentView {
  state: SessionState
  serverNow: string
  deadline?: string
  running: boolean
  finishing?: boolean
  files: Record<string, string>
  readOnly: string[]
  runs: RunView[]
  flags: Flag[]
  report?: ReportSummary
}

export type EventKind = 'hidden' | 'visible' | 'paste'
export interface SessionEvent { at: string; kind: EventKind; size: number }
export interface Hint { mutationId: string; explanation: string; question: string; source: string }

export interface AuditorView extends Omit<StudentView, 'flags' | 'report'> {
  progress: { stage: string; done: number; total: number }
  error?: string
  studentUrl?: string
  mutations: Mutation[]
  killingTests: string[][]
  hints: Hint[]
  mutated: Record<string, string>
  flags: AuditorFlag[]
  events: SessionEvent[]
  stats?: { candidates: number; killed: number; survived: number; buildErrors: number; triplesChecked: number }
  report?: Report
}

export const states: Record<SessionState, string> = {
  preparing: 'Подготовка',
  ready: 'Ожидает студента',
  running: 'В работе',
  finished: 'Завершена',
  failed: 'Ошибка подготовки',
}
