import type { ReactNode } from 'react'
import {
  ContainerOutlined,
  EyeOutlined,
  FunctionOutlined,
  NumberOutlined,
  TableOutlined,
} from '@ant-design/icons'

import type { ObjectKind } from '../api/types'

/**
 * Explorer glyph for an object kind.
 *
 * Shared by the tree and the object-list windows so the same object always
 * looks the same in both places.
 */
export function objectIcon(kind: ObjectKind): ReactNode {
  switch (kind) {
    case 'view':
    case 'materialized_view':
      return <EyeOutlined />
    case 'collection':
      return <ContainerOutlined />
    case 'sequence':
      return <NumberOutlined />
    case 'procedure':
      return <FunctionOutlined />
    default:
      return <TableOutlined />
  }
}
