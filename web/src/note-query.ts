// Rendering embedded ```note-query blocks in the preview.
//
// Three rules, all from `PLAN.md` E7.
//
// **A block never blocks the note.** The note renders first with a placeholder
// in the block's place; results arrive afterwards. A slow or failing query
// leaves a message inside the block and the rest of the note untouched.
//
// **Nothing is evaluated here.** The block's text goes to the service, which
// parses it with the same Q1 parser every search surface uses. This file has no
// query language in it, so there is nothing for a client and a server to
// disagree about — the lesson E5 recorded about Markdown links.
//
// **No markup is built from note content.** Every value that comes back is
// written with `textContent` or as an element attribute, never concatenated
// into HTML. A block is note content and note content is untrusted.
import { runNoteQuery, type NoteQueryResult, type NoteQueryRow } from './api';

/** How many blocks one note may evaluate. Beyond this the rest say so. */
export const MAX_BLOCKS_PER_NOTE = 20;

function element(tag: string, className?: string, text?: string): HTMLElement {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

/** Replaces a block's contents. Always clears first, so a re-render is clean. */
function fill(block: HTMLElement, ...children: Node[]): void {
  block.replaceChildren(...children);
}

function formatUpdated(value?: string): string {
  if (!value) return '';
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

/** Builds one result row. Links open through the ordinary preview routing. */
function rowElement(row: NoteQueryRow, fields: string[]): HTMLElement {
  const item = element('li', 'note-query-row');
  const link = element('a', 'note-query-title', row.title || row.document_id);
  // `data-app-uri` is what the preview's click handler routes on, and href="#"
  // keeps it a real link for keyboard users without navigating anywhere.
  link.setAttribute('data-app-uri', row.uri);
  link.setAttribute('href', '#');
  item.append(link);

  const meta: string[] = [];
  if (fields.includes('notebook') && row.notebook) meta.push(row.notebook);
  if (fields.includes('tags') && row.tags?.length) meta.push(row.tags.map((tag) => `#${tag}`).join(' '));
  if (fields.includes('updated') && row.updated_at) meta.push(formatUpdated(row.updated_at));
  if (meta.length) item.append(element('span', 'note-query-meta', ` · ${meta.join(' · ')}`));
  if (fields.includes('snippet') && row.snippet) {
    item.append(element('div', 'note-query-snippet', row.snippet));
  }
  return item;
}

/** Renders a completed result into its block. */
export function renderNoteQueryResult(block: HTMLElement, result: NoteQueryResult): void {
  if (result.error) {
    // The message names the problem and the block keeps its place in the note.
    // Nothing about the surrounding note changes.
    fill(
      block,
      element('div', 'note-query-error', `Query block: ${result.error}`),
    );
    block.setAttribute('data-note-query-state', 'error');
    return;
  }
  const fields = result.spec?.fields ?? ['title'];
  if (!result.rows?.length) {
    fill(block, element('div', 'note-query-status', 'No notes match this query.'));
    block.setAttribute('data-note-query-state', 'empty');
    return;
  }
  const list = element('ul', 'note-query-rows');
  for (const row of result.rows) list.append(rowElement(row, fields));
  const children: Node[] = [list];
  if (result.truncated) {
    // Truncation is always visible. A list that silently stops is a list that
    // quietly lies about the library.
    children.push(element('div', 'note-query-truncated', `Showing the first ${result.rows.length}; more notes match.`));
  }
  fill(block, ...children);
  block.setAttribute('data-note-query-state', 'ok');
}

/**
 * Finds every query block in a rendered note and evaluates it.
 *
 * Returns a cleanup that aborts anything still in flight, so navigating away
 * from a note does not let a late reply write into a different note's preview.
 */
export function renderNoteQueryBlocks(container: HTMLElement): () => void {
  const blocks = Array.from(container.querySelectorAll<HTMLElement>('[data-note-query]'));
  const controller = new AbortController();
  let cancelled = false;

  blocks.forEach((block, index) => {
    if (index >= MAX_BLOCKS_PER_NOTE) {
      fill(block, element('div', 'note-query-error', `Only the first ${MAX_BLOCKS_PER_NOTE} query blocks in a note are evaluated.`));
      block.setAttribute('data-note-query-state', 'skipped');
      return;
    }
    const source = block.getAttribute('data-note-query-source') ?? '';
    runNoteQuery(source, { signal: controller.signal })
      .then((result) => {
        if (cancelled) return;
        renderNoteQueryResult(block, result);
      })
      .catch(() => {
        if (cancelled) return;
        // The service is unreachable. Say so inside the block rather than
        // leaving "Running query…" up forever or failing the note.
        fill(block, element('div', 'note-query-error', 'Query block: the service could not be reached.'));
        block.setAttribute('data-note-query-state', 'error');
      });
  });

  return () => {
    cancelled = true;
    controller.abort();
  };
}
