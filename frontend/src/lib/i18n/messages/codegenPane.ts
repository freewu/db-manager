import type { AreaMessages } from '../types'

// source: frontend/src/components/CodegenPane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const codegenPane = {
  'codegenPane.code-copied': ['Code copied', '代码已复制', '程式碼已複製'],
  'codegenPane.nothing-to-save': ['Nothing to save', '没有可保存的内容', '沒有可儲存的內容'],
  'codegenPane.code-saved': ['Code saved', '代码已保存', '程式碼已儲存'],
  'codegenPane.language': ['Language', '语言', '語言'],
  'codegenPane.copy-the-generated-code': ['Copy the generated code', '复制生成的代码', '複製產生的程式碼'],
  'codegenPane.save-the-generated-code-to-a-file': [
    'Save the generated code to a file',
    '将生成的代码保存到文件',
    '將產生的程式碼儲存至檔案',
  ],
  'codegenPane.read-the-object-s-fields-again': [
    'Read the object\'s fields again',
    '重新读取对象的字段',
    '重新讀取物件的欄位',
  ],
  'codegenPane.fields': ['{n} field|{n} fields', '{n} 个字段|{n} 个字段', '{n} 個欄位|{n} 個欄位'],
  'codegenPane.could-not-read-the-object-s-fields': [
    'Could not read the object\'s fields',
    '无法读取对象的字段',
    '無法讀取物件的欄位',
  ],
  'codegenPane.retry': ['Retry', '重试', '重試'],
  'codegenPane.this-object-has-no-fields': ['This object has no fields', '该对象没有字段', '此物件沒有欄位'],
  'codegenPane.there-is-nothing-to-map-yet-so-the-generated': [
    'There is nothing to map yet, so the generated file is the empty shell.',
    '尚无可映射的内容，因此生成的文件只是一个空壳。',
    '尚無可對應的內容，因此產生的檔案只是一個空殼。',
  ],
  'codegenPane.reading-the-object-s-fields': [
    'Reading the object\'s fields…',
    '正在读取对象的字段…',
    '正在讀取物件的欄位…',
  ],
  'codegenPane.window-not-pointed-at-an-object': [
    'This window is not pointed at an object.',
    '该窗口未指向任何对象。',
    '此視窗未指向任何物件。',
  ],
} satisfies AreaMessages
