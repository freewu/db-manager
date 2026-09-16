import { useMemo } from 'react'
import CodeMirror from '@uiw/react-codemirror'
import {
  MSSQL,
  MySQL,
  PLSQL,
  PostgreSQL,
  SQLite,
  StandardSQL,
  sql,
} from '@codemirror/lang-sql'
import { Prec } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'

import type { DriverType } from '../api/types'

interface SqlEditorProps {
  value: string
  driver: DriverType | undefined
  theme: 'light' | 'dark'
  height: string
  onChange: (value: string) => void
  /** Invoked by Ctrl/Cmd-Enter. */
  onRun: () => void
  /** Invoked by Ctrl/Cmd-Shift-Enter (run the selection). */
  onRunSelection: () => void
  /** Receives the EditorView so callers can read the current selection. */
  onReady?: (view: EditorView) => void
}

function dialectFor(driver: DriverType | undefined) {
  switch (driver) {
    case 'mysql':
      return MySQL
    case 'postgres':
      return PostgreSQL
    case 'sqlite':
      return SQLite
    case 'sqlserver':
      return MSSQL
    case 'oracle':
      return PLSQL
    default:
      return StandardSQL
  }
}

/** CodeMirror 6 SQL editor with a driver-aware dialect. */
export function SqlEditor({
  value,
  driver,
  theme,
  height,
  onChange,
  onRun,
  onRunSelection,
  onReady,
}: SqlEditorProps) {
  const extensions = useMemo(
    () => [
      sql({ dialect: dialectFor(driver), upperCaseKeywords: true }),
      EditorView.lineWrapping,
      // Highest precedence so the shortcuts win over CodeMirror defaults.
      Prec.highest(
        keymap.of([
          {
            key: 'Mod-Enter',
            preventDefault: true,
            run: () => {
              onRun()
              return true
            },
          },
          {
            key: 'Mod-Shift-Enter',
            preventDefault: true,
            run: () => {
              onRunSelection()
              return true
            },
          },
        ]),
      ),
    ],
    [driver, onRun, onRunSelection],
  )

  return (
    <CodeMirror
      className="dm-editor"
      value={value}
      height={height}
      theme={theme}
      extensions={extensions}
      onChange={onChange}
      onCreateEditor={(view) => onReady?.(view)}
      basicSetup={{
        lineNumbers: true,
        foldGutter: false,
        highlightActiveLine: true,
        highlightActiveLineGutter: true,
        autocompletion: true,
        bracketMatching: true,
        closeBrackets: true,
        indentOnInput: true,
      }}
    />
  )
}
