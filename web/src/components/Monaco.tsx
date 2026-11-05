import { useLayoutEffect, useRef } from 'react'
import Editor, { DiffEditor, loader, type EditorProps } from '@monaco-editor/react'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import 'monaco-editor/esm/vs/editor/editor.all'
import 'monaco-editor/esm/vs/basic-languages/go/go.contribution'
import EditorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import { Loading } from './Shell'

self.MonacoEnvironment = { getWorker: () => new EditorWorker() }
loader.config({ monaco })

monaco.editor.defineTheme('live-code', {
  base: 'vs-dark',
  inherit: true,
  rules: [],
  colors: {
    'editor.background': '#0d1624',
    'editorGutter.background': '#0d1624',
    'editorLineNumber.foreground': '#7387a1',
    'editor.lineHighlightBackground': '#182538',
    'editor.selectionBackground': '#26505f',
  },
})

const options: monaco.editor.IStandaloneEditorConstructionOptions = {
  automaticLayout: true,
  minimap: { enabled: false },
  scrollBeyondLastLine: false,
  fontSize: 14,
  padding: { top: 12, bottom: 12 },
  tabSize: 4,
  insertSpaces: false,
  accessibilitySupport: 'auto',
}

function language(path: string) {
  return path.endsWith('.go') ? 'go' : 'plaintext'
}

export function CodeEditor({ file, readOnly, ...props }: EditorProps & { file: string; readOnly: boolean }) {
  return <Editor {...props} language={language(file)} height="100%" theme="live-code"
    loading={<Loading>Загружаем редактор…</Loading>}
    options={{ ...options, readOnly, ariaLabel: `Редактор файла ${file}` }} />
}

export function CodeDiff({ file, original, modified, lines }: {
  file: string; original: string; modified: string; lines: number[]
}) {
  const instance = useRef<monaco.editor.IStandaloneDiffEditor>()
  useLayoutEffect(() => () => {
    const editor = instance.current
    if (!editor) return
    const model = editor.getModel()
    // Detach before disposing models; the React wrapper disposes models first.
    editor.setModel(null)
    model?.original.dispose()
    model?.modified.dispose()
    instance.current = undefined
  }, [])
  return <DiffEditor
    keepCurrentOriginalModel
    keepCurrentModifiedModel
    original={original}
    modified={modified}
    language={language(file)}
    height="100%"
    theme="live-code"
    loading={<Loading>Загружаем сравнение…</Loading>}
    options={{
      ...options, readOnly: true, originalEditable: false, renderSideBySide: true,
      useInlineViewWhenSpaceIsLimited: true, originalAriaLabel: 'Выданный код', modifiedAriaLabel: 'Код студента',
    }}
    onMount={editor => {
      instance.current = editor
      const baseline = editor.getOriginalEditor()
      const lineCount = baseline.getModel()?.getLineCount() ?? 0
      baseline.createDecorationsCollection(lines.filter(line => line >= 1 && line <= lineCount).map(line => ({
        range: new monaco.Range(line, 1, line, 1),
        options: { isWholeLine: true, className: 'mutation-line', linesDecorationsClassName: 'mutation-gutter' },
      })))
      if (lines[0]) baseline.revealLineInCenter(lines[0])
    }}
  />
}
