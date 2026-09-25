import en_US from 'antd/locale/en_US'
import zh_CN from 'antd/locale/zh_CN'
import zh_TW from 'antd/locale/zh_TW'

import type { Language } from './types'

/**
 * The Ant Design words for a language.
 *
 * Ant Design has its own translations of the handful of phrases it says itself —
 * the pager, the "OK" of a `Modal.confirm`, the sort arrows' titles, the empty
 * text of a table. They are separate from our message tables because they are
 * not our words, and they are picked here so no component has to know them.
 */
export function antdLocaleOf(language: Language): typeof en_US {
  if (language === 'zh-CN') return zh_CN
  if (language === 'zh-TW') return zh_TW
  return en_US
}
