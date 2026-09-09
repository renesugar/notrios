// Text for the organizer's confirmations.
//
// It lives here rather than inline in a handler because the wording is the
// feature. A dialog that says "Delete this notebook?" tells the reader nothing
// they did not already know; the thing worth confirming is what happens to the
// notes inside it, and that answer comes from the service rather than from a
// guess made here.
import type { NotebookDeletionPreview, NotebookTreeNode } from './api';

/** The display name of a notebook in the tree, falling back to its ID. */
export function notebookName(tree: NotebookTreeNode[], notebookID: string): string {
  for (const node of tree) {
    if (node.id === notebookID) return node.name;
    if (node.children && node.children.length > 0) {
      const found = notebookName(node.children, notebookID);
      if (found !== notebookID) return found;
    }
  }
  return notebookID;
}

/**
 * The confirmation shown before deleting a notebook.
 *
 * Three facts, in the order they matter: how many notebooks go, that the notes
 * are not lost, and where they land. The last one is the part nobody expects —
 * a restored note has to have a notebook to return to, so the service re-homes
 * the whole subtree to the default notebook on the way to the Trash.
 */
export function notebookDeletionPrompt(preview: NotebookDeletionPreview, rehomeName: string): string {
  const lines = [`Delete the notebook “${preview.name}”?`, ''];

  if (preview.notebooks > 1) {
    const named = preview.descendant_names.join(', ');
    const suffix = preview.truncated ? `${named}, …` : named;
    lines.push(`${preview.notebooks} notebooks are removed, including ${suffix}.`);
  } else {
    lines.push('The notebook is removed.');
  }

  if (preview.notes > 0) {
    lines.push(`${preview.notes} note(s) are not deleted: they move to the Trash, where you can restore them.`);
  } else {
    lines.push('It holds no notes.');
  }
  if (preview.trashed_notes > 0) {
    lines.push(`${preview.trashed_notes} note(s) already in the Trash are also affected.`);
  }
  if (preview.notes > 0 || preview.trashed_notes > 0) {
    lines.push(`Those notes move to “${rehomeName}”, so restoring one later has somewhere to put it.`);
  }
  return lines.join('\n');
}
