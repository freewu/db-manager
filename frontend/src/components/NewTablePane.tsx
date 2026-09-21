import type { WorkspaceTab } from '../store/appStore'
import { StructureView } from './StructurePane'

/**
 * The table designer opened for a table that does not exist yet.
 *
 * It is the same window as an existing table's Structure tab — the fields grid,
 * the indexes grid and the SQL preview are the same components — with the one
 * difference that matters: nothing is read from the catalog, and Save runs the
 * CREATE TABLE the preview shows instead of an ALTER that reconciles it.
 */
export function NewTablePane({ tab }: { tab: WorkspaceTab }) {
  return <StructureView tab={tab} section="structure" creating />
}
