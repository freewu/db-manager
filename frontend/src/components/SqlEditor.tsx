import { useMemo } from 'react'
import CodeMirror from '@uiw/react-codemirror'
import { javascript } from '@codemirror/lang-javascript'
import {
  MSSQL,
  MySQL,
  PLSQL,
  PostgreSQL,
  SQLite,
  StandardSQL,
  sql,
} from '@codemirror/lang-sql'
import { Prec, type Extension } from '@codemirror/state'
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

/**
 * The highlighting for a driver's language.
 *
 * MongoDB is the odd one out: its shell is JavaScript built around a `db`
 * object, so it gets the JavaScript mode — highlighting the same text as SQL
 * would mark every call and every brace as a mistake. Everything else is SQL,
 * with the engine's own dialect.
 */
function languageFor(driver: DriverType | undefined): Extension {
  switch (driver) {
    case 'mongodb':
      return javascript()
    case 'mysql':
      return sql({ dialect: MySQL, upperCaseKeywords: true })
    case 'postgres':
      return sql({ dialect: PostgreSQL, upperCaseKeywords: true })
    case 'sqlite':
      return sql({ dialect: SQLite, upperCaseKeywords: true })
    case 'sqlserver':
      return sql({ dialect: MSSQL, upperCaseKeywords: true })
    case 'oracle':
      return sql({ dialect: PLSQL, upperCaseKeywords: true })
    default:
      return sql({ dialect: StandardSQL, upperCaseKeywords: true })
  }
}

/** CodeMirror 6 editor with a driver-aware language and run shortcuts. */
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
      languageFor(driver),
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
