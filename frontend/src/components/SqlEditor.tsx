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
  type SQLNamespace,
} from '@codemirror/lang-sql'
import { Prec, type Extension } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'

import type { DriverType, ObjectInfo } from '../api/types'
import { isMySQLFamily } from '../lib/sqlFlavor'
import { KIND_SINGULAR } from '../lib/tree'

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
  /**
   * Invoked by Ctrl/Cmd-S. Left out when there is nowhere to save to (a query
   * scratchpad, a DDL preview), in which case the shortcut does nothing rather
   * than pretending a save happened.
   */
  onSave?: () => void
  /**
   * Invoked by Ctrl/Cmd-Shift-F. Left out by callers that have nothing to
   * format with (a document store's shell), which is also how the toolbar
   * decides to disable its Format button.
   */
  onFormat?: () => void
  /** Receives the EditorView so callers can read the current selection. */
  onReady?: (view: EditorView) => void
  /**
   * The tables and views the window is about, completed by name.
   *
   * The list is the namespace's catalog, so it carries names and kinds but no
   * columns: a name completes, and `table.` then has nothing to offer. Left out
   * (or empty) the editor completes keywords only.
   */
  catalog?: readonly ObjectInfo[]
}

/**
 * The completion list for a window's namespace.
 *
 * lang-sql's `schema` is a namespace: each key is a table and its value is that
 * table's columns. The `{self, children}` form is what lets an entry say it is
 * a table or a view — the default entry is typed `type`, which says nothing —
 * while an empty child list still completes the name itself.
 */
function namespaceOf(catalog: readonly ObjectInfo[] | undefined): SQLNamespace | undefined {
  if (!catalog || catalog.length === 0) return undefined
  const namespace: Record<string, SQLNamespace> = {}
  for (const object of catalog) {
    namespace[object.name] = {
      self: {
        label: object.name,
        type: 'type',
        detail: KIND_SINGULAR[object.kind].toLowerCase(),
      },
      children: [],
    }
  }
  return namespace
}

/**
 * The highlighting for a driver's language.
 *
 * MongoDB is the odd one out: its shell is JavaScript built around a `db`
 * object, so it gets the JavaScript mode — highlighting the same text as SQL
 * would mark every call and every brace as a mistake. Everything else is SQL,
 * with the engine's own dialect.
 */
function languageFor(driver: DriverType | undefined, schema: SQLNamespace | undefined): Extension {
  if (isMySQLFamily(driver)) return sql({ dialect: MySQL, upperCaseKeywords: true, schema })
  switch (driver) {
    case 'mongodb':
      return javascript()
    case 'postgres':
      return sql({ dialect: PostgreSQL, upperCaseKeywords: true, schema })
    case 'sqlite':
      return sql({ dialect: SQLite, upperCaseKeywords: true, schema })
    case 'sqlserver':
      return sql({ dialect: MSSQL, upperCaseKeywords: true, schema })
    case 'oracle':
      return sql({ dialect: PLSQL, upperCaseKeywords: true, schema })
    default:
      return sql({ dialect: StandardSQL, upperCaseKeywords: true, schema })
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
  onSave,
  onFormat,
  onReady,
  catalog,
}: SqlEditorProps) {
  const extensions = useMemo(
    () => [
      languageFor(driver, namespaceOf(catalog)),
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
          ...(onSave
            ? [
                {
                  key: 'Mod-s',
                  preventDefault: true,
                  run: () => {
                    onSave()
                    return true
                  },
                },
              ]
            : []),
          ...(onFormat
            ? [
                {
                  key: 'Mod-Shift-f',
                  preventDefault: true,
                  run: () => {
                    onFormat()
                    return true
                  },
                },
              ]
            : []),
        ]),
      ),
    ],
    [catalog, driver, onRun, onRunSelection, onSave, onFormat],
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
