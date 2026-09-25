import type { AreaMessages } from '../types'

// source: frontend/src/lib/mock/engine.ts
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const libMockEngine = {
  'libMockEngine.argument-unclosed': [
    'an argument is missing its closing {quote}',
    '参数缺少结尾的 {quote}',
    '引數缺少結尾的 {quote}',
  ],
  'libMockEngine.comma-separated': ['arguments are separated by commas', '参数以逗号分隔', '引數以逗號分隔'],
  'libMockEngine.stray-at': [
    '\'@\' must start a placeholder name — write @@ for a literal @',
    '\'@\' 后面必须是占位符名称 —— 想要字面量 @ 请写 @@',
    '\'@\' 後面必須是佔位符名稱 —— 想要字面量 @ 請寫 @@',
  ],
  'libMockEngine.unclosed-arguments': [
    'the arguments of @{name} have no closing \')\'',
    '@{name} 的参数缺少结束的 \')\'',
    '@{name} 的參數缺少結尾的 \')\'',
  ],
  'libMockEngine.unknown-placeholder': [
    '@{name} is not a placeholder this app knows',
    '@{name} 不是本应用已知的占位符',
    '@{name} 不是本應用程式已知的佔位符',
  ],
  'libMockEngine.expands-into-itself': [
    '@{name} expands into itself ({path})',
    '@{name} 展开为自身（{path}）',
    '@{name} 展開為自身（{path}）',
  ],
  'libMockEngine.custom-takes-no-arguments': [
    '@{name} takes no arguments — a custom placeholder is a whole template',
    '@{name} 不接受参数 —— 自定义占位符本身就是一个完整模板',
    '@{name} 不接受引數 —— 自訂佔位符本身就是一個完整範本',
  ],
  'libMockEngine.empty-template': [
    '@{name} has no template to render',
    '@{name} 没有可渲染的模板',
    '@{name} 沒有可渲染的範本',
  ],
  'libMockEngine.inside-custom': [
    'in @{name}: {error}',
    '在 @{name} 中：{error}',
    '在 @{name} 中：{error}',
  ],
  'libMockEngine.wrong-argument-count': [
    '@{name} {arguments}',
    '@{name} {arguments}',
    '@{name} {arguments}',
  ],
  'libMockEngine.number-argument': [
    '@{name} expects a number as argument {index}, not “{value}”',
    '@{name} 的第 {index} 个参数应为数字，而非“{value}”',
    '@{name} 的第 {index} 個引數應為數字，而非「{value}」',
  ],
  'libMockEngine.takes-no-arguments': ['takes no arguments', '不接受参数', '不接受引數'],
  'libMockEngine.takes-arguments': [
    'takes {n} argument|takes {n} arguments',
    '需要 {n} 个参数|需要 {n} 个参数',
    '需要 {n} 個引數|需要 {n} 個引數',
  ],
  'libMockEngine.takes-arguments-between': [
    'takes between {least} and {most} arguments',
    '需要 {least} 到 {most} 个参数',
    '需要 {least} 到 {most} 個引數',
  ],
} satisfies AreaMessages
