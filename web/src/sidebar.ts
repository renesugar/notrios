// Deterministic sidebar composition (UI_DESIGN.md invariants):
//
//   1. "All notes" (builtin first-anchored search notebook) is always first.
//   2. The ordinary notebook tree, excluding the builtin Help node.
//   3. User-created (normal-anchored) search notebooks.
//   4. The builtin Help regular notebook — always immediately above Trash.
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
export const ALL_NOTES_SEARCH_ID = 'snb_all_notes';
export const TRASH_SEARCH_ID = 'snb_trash';

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

  // 2. The notebook tree, excluding the top-level builtin Help node.
  let helpNode: NotebookTreeNode | null = null;
  const walk = (nodes: NotebookTreeNode[], depth: number) => {
    for (const node of nodes) {
      if (depth === 0 && node.id === HELP_NOTEBOOK_ID) {
        helpNode = node;
        continue;
      }
      rows.push(notebookRow(node, depth));
      if (node.children && node.children.length > 0) walk(node.children, depth + 1);
    }
  };
  walk(tree, 0);

  // 3. User search notebooks.
  for (const sn of userSearch) rows.push(searchRow(sn));

  // 4/5. Any other last-anchored builtins, then Help immediately above Trash.
  for (const sn of otherLast) rows.push(searchRow(sn));
  if (helpNode !== null) rows.push(notebookRow(helpNode, 0));
  for (const sn of trash) rows.push(searchRow(sn));

  return rows;
}
