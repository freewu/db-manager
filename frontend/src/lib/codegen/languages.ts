/**
 * The languages object code can be generated in.
 *
 * Each entry is data rather than a class: a type table, a name convention, how
 * the language spells "may hold NULL", and one function that draws the file.
 * Adding a language is adding an entry — nothing else in the app names one, the
 * picker and the settings page both read the list from here.
 *
 * The type tables are deliberately shorter than the engines are: a column is
 * reduced to a kind (see `index.ts`) before a language sees it, so `varchar`,
 * `nvarchar` and `character varying` are one entry here instead of three.
 */
import {
  codeFile,
  generatedLine,
  ident,
  noteLine,
  typeWidth,
  type CodeLanguage,
  type ColumnKind,
  type RenderContext,
  type RenderedField,
} from './types'

/** Whether any of the named kinds turned up in the object. */
function hasAny(context: RenderContext, ...kinds: ColumnKind[]): boolean {
  return kinds.some((kind) => context.kinds.has(kind))
}

/**
 * A field's comment plus a NULL note, for a trailing comment.
 *
 * Only the languages that can say neither in the type use this; the rest put the
 * comment above the member and let the type carry the NULL.
 */
function fieldNote(field: RenderedField): string {
  return [field.comment, field.nullable ? 'may be NULL' : ''].filter(Boolean).join('; ')
}

/* --- shared builders ------------------------------------------------------ */

/** Go spells initialisms in full: `user_id` is `UserID`, not `UserId`. */
const GO_INITIALISMS: Record<string, string> = {
  Id: 'ID',
  Url: 'URL',
  Api: 'API',
  Uuid: 'UUID',
  Json: 'JSON',
  Sql: 'SQL',
  Http: 'HTTP',
}

function goName(name: string): string {
  return name.replace(/(Id|Url|Api|Uuid|Json|Sql|Http)(?![a-z])/g, (word) => GO_INITIALISMS[word])
}

/** The Go types that are already able to be nil. */
const GO_NILABLE = new Set(['[]byte', 'json.RawMessage', 'any'])

/** The Objective-C types that are a value rather than a pointer. */
const OBJC_SCALARS = new Set(['int32_t', 'int64_t', 'float', 'double', 'BOOL'])

/** How an Objective-C property of that type is declared owned. */
function objcAttribute(type: string): string {
  if (OBJC_SCALARS.has(type)) return 'assign'
  return type.startsWith('NSString') ? 'copy' : 'strong'
}

/** A declaration, which puts the star against the name: `NSString *email`. */
function objcDeclaration(type: string, name: string): string {
  return type.endsWith(' *') ? `${type.slice(0, -2)} *${name}` : `${type} ${name}`
}

/** The Java types, and the boxed twin each primitive needs to hold a NULL. */
const JAVA_TYPES: Partial<Record<ColumnKind, string>> = {
  int: 'int',
  long: 'long',
  float: 'float',
  double: 'double',
  decimal: 'BigDecimal',
  bool: 'boolean',
  string: 'String',
  bytes: 'byte[]',
  date: 'LocalDate',
  time: 'LocalTime',
  datetime: 'LocalDateTime',
  json: 'String',
  uuid: 'UUID',
}

const JAVA_BOXED: Record<string, string> = {
  int: 'Integer',
  long: 'Long',
  float: 'Float',
  double: 'Double',
  boolean: 'Boolean',
}

/**
 * Java, with or without Lombok.
 *
 * The two are one builder because they differ in exactly one place: Lombok's
 * `@Data` writes the accessors, and a project without Lombok needs them spelled
 * out. Everything else — the imports, the fields, the doc comments — is the
 * same file.
 */
function javaLanguage(options: {
  id: string
  label: string
  style: 'camel'
  lombok: boolean
}): CodeLanguage {
  return {
    id: options.id,
    label: options.label,
    ext: 'java',
    style: options.style,
    types: JAVA_TYPES,
    fallback: 'Object',
    nullable: (type) => JAVA_BOXED[type] ?? type,
    render: (context) => {
      const imports = [
        context.kinds.has('decimal') ? 'import java.math.BigDecimal;' : false,
        context.kinds.has('date') ? 'import java.time.LocalDate;' : false,
        context.kinds.has('time') ? 'import java.time.LocalTime;' : false,
        context.kinds.has('datetime') ? 'import java.time.LocalDateTime;' : false,
        context.kinds.has('uuid') ? 'import java.util.UUID;' : false,
        options.lombok ? 'import lombok.Data;' : false,
      ]
      const members = context.fields.map((field) => {
        const doc = field.comment ? `    /** ${field.comment} */\n` : ''
        const field0 = `${doc}    private ${field.type} ${field.name};`
        if (options.lombok) return field0
        const suffix = ident(field.name, 'pascal')
        return [
          field0,
          '',
          `    public ${field.type} get${suffix}() {`,
          `        return ${field.name};`,
          '    }',
          '',
          `    public void set${suffix}(${field.type} ${field.name}) {`,
          `        this.${field.name} = ${field.name};`,
          '    }',
        ].join('\n')
      })
      return codeFile([
        generatedLine(context, '//'),
        imports,
        '',
        context.comment ? `/** ${context.comment} */` : false,
        options.lombok ? '@Data' : false,
        `public class ${context.name} {`,
        members.length ? members.join('\n\n') : false,
        '}',
      ])
    },
  }
}
/* --- the languages -------------------------------------------------------- */

export const CODE_LANGUAGES: CodeLanguage[] = [
  {
    id: 'python',
    label: 'Python',
    ext: 'py',
    style: 'snake',
    types: {
      int: 'int',
      long: 'int',
      float: 'float',
      double: 'float',
      decimal: 'Decimal',
      bool: 'bool',
      string: 'str',
      bytes: 'bytes',
      date: 'date',
      time: 'time',
      datetime: 'datetime',
      json: 'Any',
      uuid: 'UUID',
    },
    fallback: 'Any',
    nullable: (type) => `Optional[${type}]`,
    render: (context) => {
      const typing = [
        hasAny(context, 'json', 'unknown') ? 'Any' : '',
        context.nullable ? 'Optional' : '',
      ].filter(Boolean)
      const members = context.fields.map((field) => {
        const doc = field.comment ? `    """${field.comment}"""\n` : ''
        return `${doc}    ${field.name}: ${field.type}`
      })
      return codeFile([
        generatedLine(context, '#'),
        '',
        'from dataclasses import dataclass',
        context.kinds.has('decimal') ? 'from decimal import Decimal' : false,
        hasAny(context, 'date', 'time', 'datetime')
          ? 'from datetime import date, datetime, time'
          : false,
        context.kinds.has('uuid') ? 'from uuid import UUID' : false,
        typing.length ? `from typing import ${typing.join(', ')}` : false,
        '',
        '@dataclass',
        `class ${context.name}:`,
        context.comment ? `    """${context.comment}"""` : false,
        members.length ? members : '    pass',
      ])
    },
  },

  {
    id: 'c',
    label: 'C',
    ext: 'c',
    style: 'snake',
    types: {
      int: 'int32_t',
      long: 'int64_t',
      float: 'float',
      double: 'double',
      decimal: 'double',
      bool: 'bool',
      string: 'char *',
      bytes: 'uint8_t *',
      date: 'time_t',
      time: 'time_t',
      datetime: 'time_t',
      json: 'char *',
      uuid: 'char *',
    },
    fallback: 'void *',
    render: (context) => {
      const name = `${ident(context.object, 'snake')}_t`
      const width = typeWidth(context.fields)
      const members = context.fields.map((field) => {
        const note = fieldNote(field)
        return `    ${field.type.padEnd(width)} ${field.name};${note ? ` /* ${note} */` : ''}`
      })
      return codeFile([
        generatedLine(context, '//'),
        '#include <stdint.h>',
        context.kinds.has('bool') ? '#include <stdbool.h>' : false,
        hasAny(context, 'date', 'time', 'datetime') ? '#include <time.h>' : false,
        '',
        context.comment ? `/* ${context.comment} */` : false,
        `typedef struct ${name} {`,
        members,
        `} ${name};`,
      ])
    },
  },

  {
    id: 'cpp',
    label: 'C++',
    ext: 'cpp',
    style: 'snake',
    types: {
      int: 'std::int32_t',
      long: 'std::int64_t',
      float: 'float',
      double: 'double',
      decimal: 'double',
      bool: 'bool',
      string: 'std::string',
      bytes: 'std::vector<std::uint8_t>',
      date: 'std::string',
      time: 'std::string',
      datetime: 'std::string',
      json: 'std::string',
      uuid: 'std::string',
    },
    fallback: 'std::any',
    nullable: (type) => `std::optional<${type}>`,
    render: (context) => {
      const width = typeWidth(context.fields)
      const members = context.fields.map((field) => {
        const note = noteLine(field, '    //')
        const member = `    ${field.type.padEnd(width)} ${field.name};`
        return note ? `${note}\n${member}` : member
      })
      const includes = [
        hasAny(context, 'int', 'long', 'bytes') ? '#include <cstdint>' : false,
        context.kinds.has('bytes') ? '#include <vector>' : false,
        context.kinds.has('unknown') ? '#include <any>' : false,
        context.nullable ? '#include <optional>' : false,
        hasAny(context, 'string', 'date', 'time', 'datetime', 'json', 'uuid')
          ? '#include <string>'
          : false,
      ]
      return codeFile([
        generatedLine(context, '//'),
        includes,
        '',
        context.comment ? `/** ${context.comment} */` : false,
        `struct ${context.name} {`,
        members,
        '};',
      ])
    },
  },

  javaLanguage({
    id: 'java',
    label: 'Java (Lombok)',
    style: 'camel',
    lombok: true,
  }),
  javaLanguage({
    id: 'java_plain',
    label: 'Java',
    style: 'camel',
    lombok: false,
  }),

  {
    id: 'csharp',
    label: 'C#',
    ext: 'cs',
    style: 'pascal',
    types: {
      int: 'int',
      long: 'long',
      float: 'float',
      double: 'double',
      decimal: 'decimal',
      bool: 'bool',
      string: 'string',
      bytes: 'byte[]',
      date: 'DateOnly',
      time: 'TimeOnly',
      datetime: 'DateTime',
      json: 'string',
      uuid: 'Guid',
    },
    fallback: 'object',
    nullable: (type) => `${type}?`,
    render: (context) => {
      const members = context.fields.map((field) => {
        const doc = field.comment ? `    /// ${field.comment}\n` : ''
        return `${doc}    public ${field.type} ${field.name} { get; set; }`
      })
      return codeFile([
        generatedLine(context, '//'),
        hasAny(context, 'date', 'time', 'datetime', 'uuid') ? 'using System;' : false,
        '',
        context.comment ? `/// ${context.comment}` : false,
        `public class ${context.name}`,
        '{',
        members,
        '}',
      ])
    },
  },

  {
    id: 'javascript',
    label: 'JavaScript',
    ext: 'js',
    style: 'camel',
    types: {
      int: 'number',
      long: 'number',
      float: 'number',
      double: 'number',
      decimal: 'number',
      bool: 'boolean',
      string: 'string',
      bytes: 'Uint8Array',
      // A calendar date and a clock time have no instant to be: a driver hands
      // over the text it was sent, and only a timestamp becomes a Date.
      date: 'string',
      time: 'string',
      datetime: 'Date',
      json: 'object',
      uuid: 'string',
    },
    fallback: '*',
    nullable: (type) => `?${type}`,
    render: (context) => {
      const members = context.fields.map((field) => {
        const comment = field.comment ? `   * ${field.comment}\n` : ''
        return `  /**\n${comment}   * @type {${field.type}}\n   */\n  ${field.name}`
      })
      return codeFile([
        generatedLine(context, '//'),
        '',
        context.comment ? `/** ${context.comment} */` : false,
        `export class ${context.name} {`,
        members.length ? members.join('\n\n') : false,
        '',
        '  constructor(init = {}) {',
        '    Object.assign(this, init)',
        '  }',
        '}',
      ])
    },
  },

  {
    id: 'typescript',
    label: 'TypeScript',
    ext: 'ts',
    style: 'camel',
    types: {
      int: 'number',
      long: 'number',
      float: 'number',
      double: 'number',
      decimal: 'number',
      bool: 'boolean',
      string: 'string',
      bytes: 'Uint8Array',
      date: 'string',
      time: 'string',
      datetime: 'Date',
      json: 'Record<string, unknown>',
      uuid: 'string',
    },
    fallback: 'unknown',
    nullable: (type) => `${type} | null`,
    render: (context) => {
      const members = context.fields.map((field) => {
        const doc = field.comment ? `  /** ${field.comment} */\n` : ''
        return `${doc}  ${field.name}: ${field.type}`
      })
      return codeFile([
        generatedLine(context, '//'),
        '',
        context.comment ? `/** ${context.comment} */` : false,
        `export interface ${context.name} {`,
        members.length ? members.join('\n\n') : false,
        '}',
      ])
    },
  },

  {
    id: 'rust',
    label: 'Rust',
    ext: 'rs',
    style: 'snake',
    types: {
      int: 'i32',
      long: 'i64',
      float: 'f32',
      double: 'f64',
      decimal: 'f64',
      bool: 'bool',
      string: 'String',
      bytes: 'Vec<u8>',
      date: 'NaiveDate',
      time: 'NaiveTime',
      datetime: 'DateTime<Utc>',
      json: 'serde_json::Value',
      uuid: 'Uuid',
    },
    fallback: 'serde_json::Value',
    nullable: (type) => `Option<${type}>`,
    render: (context) => {
      const members = context.fields.map((field) => {
        const doc = field.comment ? `    /// ${field.comment}\n` : ''
        return `${doc}    pub ${field.name}: ${field.type},`
      })
      const chrono = [
        context.kinds.has('date') ? 'NaiveDate' : '',
        context.kinds.has('time') ? 'NaiveTime' : '',
        context.kinds.has('datetime') ? 'DateTime, Utc' : '',
      ].filter(Boolean)
      const uses = [
        chrono.length ? `use chrono::{${chrono.join(', ')}};` : false,
        context.kinds.has('uuid') ? 'use uuid::Uuid;' : false,
      ]
      const crates = [
        chrono.length ? 'chrono 0.4' : '',
        hasAny(context, 'json', 'unknown') ? 'serde_json 1' : '',
        context.kinds.has('uuid') ? 'uuid 1' : '',
      ].filter(Boolean)
      return codeFile([
        generatedLine(context, '//'),
        crates.length ? `// Crates: ${crates.join(', ')}.` : false,
        uses,
        '',
        context.comment ? `/// ${context.comment}` : false,
        '#[derive(Debug, Clone)]',
        `pub struct ${context.name} {`,
        members,
        '}',
      ])
    },
  },

  {
    id: 'php',
    label: 'PHP',
    ext: 'php',
    style: 'camel',
    types: {
      int: 'int',
      long: 'int',
      float: 'float',
      double: 'float',
      decimal: 'float',
      bool: 'bool',
      string: 'string',
      bytes: 'string',
      date: 'string',
      time: 'string',
      datetime: 'string',
      json: 'array',
      uuid: 'string',
    },
    fallback: 'mixed',
    // `mixed` already holds null, and PHP refuses `?mixed` outright.
    nullable: (type) => (type === 'mixed' ? type : `?${type}`),
    render: (context) => {
      const members = context.fields.map((field) => {
        const doc = `    /** @var ${field.type}${field.comment ? ` ${field.comment}` : ''} */`
        const property = `    public ${field.type} $${field.name}${field.nullable ? ' = null' : ''};`
        return `${doc}\n${property}`
      })
      return codeFile([
        '<?php',
        generatedLine(context, '//'),
        '',
        context.comment ? `/** ${context.comment} */` : false,
        `class ${context.name}`,
        '{',
        members.length ? members.join('\n\n') : false,
        '}',
      ])
    },
  },

  {
    id: 'go',
    label: 'Go',
    ext: 'go',
    style: 'pascal',
    types: {
      int: 'int32',
      long: 'int64',
      float: 'float32',
      double: 'float64',
      decimal: 'float64',
      bool: 'bool',
      string: 'string',
      bytes: '[]byte',
      date: 'time.Time',
      time: 'time.Time',
      datetime: 'time.Time',
      json: 'json.RawMessage',
      uuid: 'string',
    },
    fallback: 'any',
    // A slice, a map and an interface already have a nil; everything else needs
    // a pointer before it can say "no value".
    nullable: (type) => (GO_NILABLE.has(type) ? type : `*${type}`),
    render: (context) => {
      // gofmt aligns a run of fields into columns and starts a new run at each
      // commented one, so the generated file is already what `gofmt -w` would
      // leave behind — the columns are lined up the same way.
      const members: string[] = []
      let run: RenderedField[] = []
      const flush = () => {
        const names = Math.max(0, ...run.map((field) => goName(field.name).length))
        const types = Math.max(0, ...run.map((field) => field.type.length))
        for (const field of run) {
          const tag = `\`json:"${field.column}${field.nullable ? ',omitempty' : ''}" db:"${field.column}"\``
          members.push(`\t${goName(field.name).padEnd(names)} ${field.type.padEnd(types)} ${tag}`)
        }
        run = []
      }
      for (const field of context.fields) {
        if (field.comment) {
          flush()
          members.push(`\t// ${field.comment}`)
        }
        run.push(field)
      }
      flush()
      const imports = [
        context.kinds.has('json') ? '\t"encoding/json"' : false,
        hasAny(context, 'date', 'time', 'datetime') ? '\t"time"' : false,
      ].filter((line): line is string => Boolean(line))
      return codeFile([
        generatedLine(context, '//'),
        'package model',
        '',
        imports.length ? ['import (', imports, ')'] : false,
        '',
        `// ${context.name} is a row of ${context.qualified}.`,
        context.comment ? `// ${context.comment}` : false,
        `type ${context.name} struct {`,
        members,
        '}',
      ])
    },
  },

  {
    id: 'ruby',
    label: 'Ruby',
    ext: 'rb',
    style: 'snake',
    types: {
      int: 'Integer',
      long: 'Integer',
      float: 'Float',
      double: 'Float',
      decimal: 'BigDecimal',
      bool: 'true, false',
      string: 'String',
      bytes: 'String',
      date: 'Date',
      time: 'Time',
      datetime: 'Time',
      json: 'Hash',
      uuid: 'String',
    },
    fallback: 'Object',
    // A Ruby comment is the only place a type can be written, so "nil" is part
    // of the union rather than of the type.
    nullable: (type) => `${type}, nil`,
    render: (context) => {
      const members = context.fields.map(
        (field) => `  # @return [${field.type}]\n  attr_accessor :${field.name}`,
      )
      const keywords = context.fields.map((field) => `${field.name}: nil`)
      const assigns = context.fields.map((field) => `    @${field.name} = ${field.name}`)
      return codeFile([
        generatedLine(context, '#'),
        context.kinds.has('decimal') ? "require 'bigdecimal'" : false,
        context.kinds.has('date') ? "require 'date'" : false,
        '',
        context.comment ? `# ${context.comment}` : false,
        `class ${context.name}`,
        members.length ? [members.join('\n\n'), ''] : false,
        `  def initialize(${keywords.join(', ')})`,
        assigns.length ? assigns : '    # nothing to assign',
        '  end',
        'end',
      ])
    },
  },

  {
    id: 'swift',
    label: 'Swift',
    ext: 'swift',
    style: 'camel',
    types: {
      int: 'Int32',
      long: 'Int64',
      float: 'Float',
      double: 'Double',
      decimal: 'Decimal',
      bool: 'Bool',
      string: 'String',
      bytes: 'Data',
      date: 'Date',
      time: 'String',
      datetime: 'Date',
      json: 'String',
      uuid: 'UUID',
    },
    fallback: 'String',
    nullable: (type) => `${type}?`,
    render: (context) => {
      const members = context.fields.map((field) => {
        const doc = field.comment ? `    /// ${field.comment}\n` : ''
        return `${doc}    let ${field.name}: ${field.type}`
      })
      return codeFile([
        generatedLine(context, '//'),
        'import Foundation',
        '',
        context.comment ? `/// ${context.comment}` : false,
        `struct ${context.name}: Codable {`,
        members,
        '}',
      ])
    },
  },

  {
    id: 'perl',
    label: 'Perl',
    ext: 'pl',
    style: 'snake',
    // Perl has one kind of value, so its type table is what a reader would
    // write in the comment next to the assignment.
    types: {
      int: 'integer',
      long: 'integer',
      float: 'number',
      double: 'number',
      decimal: 'decimal',
      bool: 'boolean',
      string: 'string',
      bytes: 'string',
      date: 'string',
      time: 'string',
      datetime: 'string',
      json: 'hash ref',
      uuid: 'string',
    },
    fallback: 'scalar',
    render: (context) => {
      const width = Math.max(0, ...context.fields.map((field) => field.name.length))
      const members = context.fields.map((field) => {
        const note = [field.type, field.nullable ? 'may be NULL' : '', field.comment]
          .filter(Boolean)
          .join(', ')
        const assignment = `${field.name.padEnd(width)} => $args{${field.name}},`
        return `        ${assignment}${note ? `  # ${note}` : ''}`
      })
      return codeFile([
        generatedLine(context, '#'),
        `package ${context.name};`,
        '',
        'use strict;',
        'use warnings;',
        context.comment ? `\n# ${context.comment}` : false,
        '',
        'sub new {',
        '    my ($class, %args) = @_;',
        '    my $self = {',
        members.length ? members : false,
        '    };',
        '    return bless $self, $class;',
        '}',
        '',
        '1;',
      ])
    },
  },

  {
    id: 'objectivec',
    label: 'Objective-C',
    ext: 'h',
    style: 'camel',
    types: {
      int: 'int32_t',
      long: 'int64_t',
      float: 'float',
      double: 'double',
      decimal: 'NSDecimalNumber *',
      bool: 'BOOL',
      string: 'NSString *',
      bytes: 'NSData *',
      date: 'NSDate *',
      time: 'NSString *',
      datetime: 'NSDate *',
      json: 'NSString *',
      uuid: 'NSUUID *',
    },
    fallback: 'id',
    // Only a scalar cannot hold nil, so only a scalar changes type to do it.
    nullable: (type) => (OBJC_SCALARS.has(type) ? 'NSNumber *' : type),
    render: (context) => {
      const members = context.fields.map((field) => {
        const doc = field.comment ? `/** ${field.comment} */\n` : ''
        const property = `${objcDeclaration(field.type, field.name)};`
        return `${doc}@property (nonatomic, ${objcAttribute(field.type)}) ${property}`
      })
      return codeFile([
        generatedLine(context, '//'),
        '#import <Foundation/Foundation.h>',
        '',
        context.comment ? `/** ${context.comment} */` : false,
        `@interface ${context.name} : NSObject`,
        members.length ? members.join('\n\n') : false,
        '',
        '@end',
      ])
    },
  },

  {
    id: 'julia',
    label: 'Julia',
    ext: 'jl',
    style: 'snake',
    types: {
      int: 'Int32',
      long: 'Int64',
      float: 'Float32',
      double: 'Float64',
      decimal: 'Float64',
      bool: 'Bool',
      string: 'String',
      bytes: 'Vector{UInt8}',
      date: 'Dates.Date',
      time: 'Dates.Time',
      datetime: 'Dates.DateTime',
      json: 'Dict{String, Any}',
      uuid: 'Base.UUID',
    },
    fallback: 'Any',
    // `Missing` is Julia's NULL, and a field that may hold it has to say so.
    nullable: (type) => `Union{Missing, ${type}}`,
    render: (context) => {
      const members = context.fields.map((field) => {
        const doc = field.comment ? `    # ${field.comment}\n` : ''
        return `${doc}    ${field.name}::${field.type}`
      })
      return codeFile([
        generatedLine(context, '#'),
        hasAny(context, 'date', 'time', 'datetime') ? 'using Dates' : false,
        '',
        context.comment ? `# ${context.comment}` : false,
        `struct ${context.name}`,
        members,
        'end',
      ])
    },
  },

  {
    id: 'kotlin',
    label: 'Kotlin',
    ext: 'kt',
    style: 'camel',
    types: {
      int: 'Int',
      long: 'Long',
      float: 'Float',
      double: 'Double',
      decimal: 'BigDecimal',
      bool: 'Boolean',
      string: 'String',
      bytes: 'ByteArray',
      date: 'LocalDate',
      time: 'LocalTime',
      datetime: 'LocalDateTime',
      json: 'String',
      uuid: 'UUID',
    },
    fallback: 'Any',
    nullable: (type) => `${type}?`,
    render: (context) => {
      const members = context.fields.map((field) => {
        const doc = field.comment ? `    /** ${field.comment} */\n` : ''
        return `${doc}    val ${field.name}: ${field.type}${field.nullable ? ' = null' : ''},`
      })
      const imports = [
        context.kinds.has('decimal') ? 'java.math.BigDecimal' : '',
        context.kinds.has('date') ? 'java.time.LocalDate' : '',
        context.kinds.has('time') ? 'java.time.LocalTime' : '',
        context.kinds.has('datetime') ? 'java.time.LocalDateTime' : '',
        context.kinds.has('uuid') ? 'java.util.UUID' : '',
      ]
        .filter(Boolean)
        .map((type) => `import ${type}`)
      return codeFile([
        generatedLine(context, '//'),
        imports,
        '',
        context.comment ? `/** ${context.comment} */` : false,
        // A data class needs a parameter, so an object with no fields is the
        // plain class Kotlin would have used anyway.
        members.length
          ? [`data class ${context.name}(`, members, ')']
          : `class ${context.name}`,
      ])
    },
  },

  {
    id: 'erlang',
    label: 'Erlang',
    ext: 'erl',
    // A module's file is named after the module, and a module name is an atom.
    fileStyle: 'snake',
    style: 'snake',
    types: {
      int: 'integer()',
      long: 'integer()',
      float: 'float()',
      double: 'float()',
      decimal: 'float()',
      bool: 'boolean()',
      string: 'binary()',
      bytes: 'binary()',
      date: 'calendar:date()',
      time: 'calendar:time()',
      datetime: 'calendar:datetime()',
      json: 'map()',
      uuid: 'binary()',
    },
    fallback: 'term()',
    nullable: (type) => `${type} | undefined`,
    render: (context) => {
      const name = ident(context.object, 'snake')
      const members = context.fields.map((field, index) => {
        const comma = index < context.fields.length - 1 ? ',' : ''
        const value = field.nullable ? ' = undefined' : ''
        const note = field.comment ? ` % ${field.comment}` : ''
        return `    ${field.name}${value} :: ${field.type}${comma}${note}`
      })
      return codeFile([
        generatedLine(context, '%%'),
        `-module(${name}).`,
        '',
        '-export([new/0]).',
        context.comment ? `\n%% ${context.comment}` : false,
        '',
        `-record(${name}, {`,
        members,
        '}).',
        '',
        `-spec new() -> #${name}{}.`,
        'new() ->',
        `    #${name}{}.`,
      ])
    },
  },

  {
    id: 'lua',
    label: 'Lua',
    ext: 'lua',
    style: 'snake',
    types: {
      int: 'integer',
      long: 'integer',
      float: 'number',
      double: 'number',
      decimal: 'number',
      bool: 'boolean',
      string: 'string',
      bytes: 'string',
      date: 'string',
      time: 'string',
      datetime: 'string',
      json: 'table',
      uuid: 'string',
    },
    fallback: 'any',
    render: (context) => {
      // The types live in EmmyLua annotations: Lua itself has no place to put
      // one, and an editor that reads them completes the fields.
      const annotations = [
        context.comment ? `---${context.comment}` : false,
        `---@class ${context.name}`,
        ...context.fields.map((field) => {
          const optional = field.nullable ? '?' : ''
          const note = field.comment ? ` ${field.comment}` : ''
          return `---@field ${field.name}${optional} ${field.type}${note}`
        }),
      ]
      return codeFile([
        generatedLine(context, '--'),
        '',
        annotations,
        `local ${context.name} = {}`,
        `${context.name}.__index = ${context.name}`,
        '',
        '---@param fields table<string, any>|nil',
        `---@return ${context.name}`,
        `function ${context.name}.new(fields)`,
        `  local self = setmetatable({}, ${context.name})`,
        '  for key, value in pairs(fields or {}) do',
        '    self[key] = value',
        '  end',
        '  return self',
        'end',
        '',
        `return ${context.name}`,
      ])
    },
  },
]
