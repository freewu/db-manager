import type { AreaMessages } from '../types'

// source: frontend/src/lib/mock/custom.ts
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const libMockCustom = {
  'libMockCustom.give-it-a-name': ['Give the placeholder a name', '为占位符命名', '為佔位符命名'],
  'libMockCustom.name-rules': [
    'A name starts with a letter or _ and holds only letters, digits and _',
    '名称以字母或 _ 开头，只能包含字母、数字和 _',
    '名稱以字母或 _ 開頭，只能包含字母、數字和 _',
  ],
  'libMockCustom.name-too-long': ['At most {max} characters', '最多 {max} 个字符', '最多 {max} 個字元'],
  'libMockCustom.built-in-name': [
    '@{name} is already a built-in placeholder',
    '@{name} 已是内置占位符',
    '@{name} 已是內建佔位符',
  ],
  'libMockCustom.name-taken': ['@{name} already exists', '@{name} 已存在', '@{name} 已存在'],
  'libMockCustom.write-the-template': [
    'Write the template this placeholder stands for',
    '编写该占位符代表的模板',
    '撰寫該佔位符代表的範本',
  ],
  'libMockCustom.template-too-long': ['At most {max} characters', '最多 {max} 个字符', '最多 {max} 個字元'],
  'libMockCustom.description-too-long': [
    'At most {max} characters',
    '最多 {max} 个字符',
    '最多 {max} 個字元',
  ],
} satisfies AreaMessages
