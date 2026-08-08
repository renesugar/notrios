// Deterministic sidebar composition (UI_DESIGN.md invariants):
//
//   1. "All notes" (builtin first-anchored search notebook) is always first.
//   2. The ordinary notebook tree, excluding the read-only builtin nodes.
//   3. User-created (normal-anchored) search notebooks.
//   4. The read-only builtin notebooks — Reports, then Help — always
//      immediately above Trash, in the order READ_ONLY_NOTEBOOK_IDS gives.
//   5. "Trash" (builtin last-anchored search notebook) is always last.
//
// Ordering uses stable builtin IDs, never localized or user-visible names, so
// user notebooks named around "Help" or unusual API response order cannot
// displace the builtin rows. Unknown anchored search notebooks are kept
// deliberately: extra "first" anchors follow All notes; extra "last" anchors
// precede Trash (and stay above Help so Help+Trash remain adjacent... see
// composeSidebar for the exact rule).
import type { NotebookTreeNode, SearchNotebook } from './api';

export const HELP_NOTEBOOK_ID = 'nb_help';
export const REPORTS_NOTEBOOK_ID = 'nb_reports';
export const DEFAULT_NOTEBOOK_ID = 'nb_notes';

/**
 * Notebooks whose notes the service authors and nobody may edit, in the order
 * they sit above Trash. Mirrors `store.ReadOnlyNotebookIDs()`.
 *
 * Not the same set as "undeletable", which also contains the default Notes
 * notebook — see `isDeletableNotebookRow`, which needs both checks.
 */
export const READ_ONLY_NOTEBOOK_IDS = [REPORTS_NOTEBOOK_ID, HELP_NOTEBOOK_ID];
export const ALL_NOTES_SEARCH_ID = 'snb_all_notes';
export const TRASH_SEARCH_ID = 'snb_trash';

/**
 * The notebook a new note should be created into, given the selected row.
 *
 * `null` means "let the service use the default Notes notebook", which is the
 * right answer — not a fallback — for "All notes", a query-backed search
 * notebook, and the read-only Help notebook. A new note has to land somewhere,
 * and those three name a view rather than a place.
 */
export function creationTargetFor(row: SidebarRow | null | undefined): string | null {
  if (!row || row.kind !== 'notebook' || row.builtin) return null;
  return row.id;
}

/** One selectable destination in the notebook picker. */
export interface NotebookOption {
  id: string;
  name: string;
  depth: number;
}

/**
 * Flattens the notebook tree into pickable destinations, in sidebar order.
 *
 * Builtin notebooks are omitted: the service refuses to move a note into or out
 * of Help, so offering it would be offering a guaranteed 403. Mirroring the
 * server rule beats discovering it.
 */
export function notebookOptions(tree: NotebookTreeNode[], depth = 0): NotebookOption[] {
  const options: NotebookOption[] = [];
  for (const node of tree) {
    if (node.builtin) continue;
    options.push({ id: node.id, name: node.name, depth });
    if (node.children?.length) options.push(...notebookOptions(node.children, depth + 1));
  }
  return options;
}

/**
 * Whether a sidebar row offers a delete affordance.
 *
 * The service refuses to delete builtin notebooks and the default one, and a
 * button that always fails is worse than no button. This mirrors that rule
 * rather than replacing it: the service is still the authority, and a delete
 * that gets through here is still checked there.
 */
export function isDeletableNotebookRow(row: SidebarRow): boolean {
  return row.kind === 'notebook' && !row.builtin && row.id !== DEFAULT_NOTEBOOK_ID;
}

export interface SidebarRow {
  /** Stable row identity for selection/active tracking. */
  id: string;
  label: string;
  emoji: string;
  /** Tree indentation depth; search notebooks are always depth 0. */
  depth: number;
  /** The query executed when the row is activated. */
  query: string;
  builtin: boolean;
  kind: 'search-notebook' | 'notebook';
}

function searchRow(sn: SearchNotebook): SidebarRow {
  return {
    id: sn.id,
    label: sn.name,
    emoji: sn.icon_emoji ?? '',
    depth: 0,
    query: sn.query ?? '',
    builtin: sn.builtin,
    kind: 'search-notebook',
  };
}

function notebookRow(node: NotebookTreeNode, depth: number): SidebarRow {
  return {
    id: node.id,
    label: node.name,
    emoji: node.icon_emoji ?? '',
    depth,
    query: `notebook:"${node.name}"`,
    builtin: node.builtin,
    kind: 'notebook',
  };
}

export function composeSidebar(tree: NotebookTreeNode[], searchNotebooks: SearchNotebook[]): SidebarRow[] {
  const rows: SidebarRow[] = [];

  const allNotes = searchNotebooks.filter((sn) => sn.id === ALL_NOTES_SEARCH_ID);
  const trash = searchNotebooks.filter((sn) => sn.id === TRASH_SEARCH_ID);
  const otherFirst = searchNotebooks.filter((sn) => sn.sort_anchor === 'first' && sn.id !== ALL_NOTES_SEARCH_ID);
  const otherLast = searchNotebooks.filter((sn) => sn.sort_anchor === 'last' && sn.id !== TRASH_SEARCH_ID);
  const userSearch = searchNotebooks.filter(
    (sn) => sn.sort_anchor !== 'first' && sn.sort_anchor !== 'last',
  );

  // 1. All notes, then any other first-anchored builtins (deliberate, not dropped).
  for (const sn of allNotes) rows.push(searchRow(sn));
  for (const sn of otherFirst) rows.push(searchRow(sn));

  // 2. The notebook tree, excluding the top-level read-only builtin nodes.
  const readOnlyNodes = new Map<string, NotebookTreeNode>();
  const walk = (nodes: NotebookTreeNode[], depth: number) => {
    for (const node of nodes) {
      if (depth === 0 && READ_ONLY_NOTEBOOK_IDS.includes(node.id)) {
        readOnlyNodes.set(node.id, node);
        continue;
      }
      rows.push(notebookRow(node, depth));
      if (node.children && node.children.length > 0) walk(node.children, depth + 1);
    }
  };
  walk(tree, 0);

  // 3. User search notebooks.
  for (const sn of userSearch) rows.push(searchRow(sn));

  // 4/5. Any other last-anchored builtins, then the read-only builtins
  // immediately above Trash.
  for (const sn of otherLast) rows.push(searchRow(sn));
  for (const id of READ_ONLY_NOTEBOOK_IDS) {
    const node = readOnlyNodes.get(id);
    if (node) rows.push(notebookRow(node, 0));
  }
  for (const sn of trash) rows.push(searchRow(sn));

  return rows;
}
