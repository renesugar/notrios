// Converting a pasted HTML table into a Markdown pipe table.
//
// Pasting an HTML table already *looks* right — the preview renders it, and the
// sanitizer strips scripts, handlers, and inline styles. What lands in the note
// source is still raw HTML, and that is where it costs:
//
//   * `internal/markdownblocks` sees the table as paragraphs, so nothing in it
//     is block-addressable;
//   * `internal/markdownlinks` does not read `<a href>` inside HTML, so those
//     links are invisible to the graph, to lint, and to `notriosctl fix`;
//   * a publication handoff carries the raw HTML downstream to movenotes-v3,
//     Obsidian, and Quartz — the least portable thing a note can hold.
//
// Converting on paste puts the content into the format the rest of the system
// already understands.
//
// **This module refuses far more than it converts, on purpose.** A mangled
// table is worse than an HTML one: the HTML at least renders. Every rejection
// below returns a reason, and the caller then lets the ordinary paste happen.
// It is the same rule E3's fix and P7's rewriter follow for a span they cannot
// place exactly.

/** Why a paste was left alone. Returned rather than thrown; none is an error. */
export type TableRejection =
  | 'no-table'
  | 'multiple-tables'
  | 'content-outside-table'
  | 'empty-table'
  | 'ragged-rows'
  | 'merged-cells'
  | 'nested-block'
  | 'multiline-cell';

export type TableConversion =
  | { converted: true; markdown: string; rows: number; columns: number }
  | { converted: false; reason: TableRejection };

/**
 * Elements that make a cell something a pipe table cannot hold. A nested table
 * has no pipe-table representation at all; the rest carry structure that would
 * be silently flattened into a sentence.
 */
const REJECTED_IN_CELL = new Set([
  'TABLE', 'UL', 'OL', 'LI', 'PRE', 'BLOCKQUOTE', 'HR',
  'H1', 'H2', 'H3', 'H4', 'H5', 'H6',
]);

/**
 * Elements that only wrap. Real pastes are full of them — Google Docs puts a
 * `<p>` in every cell, spreadsheets add `<span>` and `<font>` — and treating
 * them as structure would refuse nearly every genuine paste.
 */
const TRANSPARENT_WRAPPERS = new Set(['P', 'DIV', 'SPAN', 'FONT', 'SECTION', 'ARTICLE']);

/** Escapes the characters that would break out of a table cell. */
function escapeCell(text: string): string {
  return text
    .replace(/\\/g, '\\\\')
    .replace(/\|/g, '\\|')
    .replace(/\s+/g, ' ')
    .trim();
}

/**
 * A link is emitted as Markdown only when its target survives the syntax: a
 * space ends an unquoted URL and an unescaped `)` closes the link early. Both
 * lessons are already recorded in E1a and E3. When the href will not survive,
 * the text is kept and the link dropped — losing a link is recoverable, writing
 * a broken one is not obviously so.
 */
function linkMarkdown(href: string, text: string): string | null {
  const target = href.trim();
  if (!target || /[\s()<>]/.test(target)) return null;
  if (!/^(https?:|mailto:|document:|resource:|notrios:|#|\/)/i.test(target)) return null;
  return `[${text}](${target})`;
}

/**
 * Renders one cell's inline content. Returns null when the cell holds something
 * a pipe table cannot represent.
 */
function renderCell(cell: Element): { text: string } | { reason: TableRejection } {
  let out = '';
  let rejection: TableRejection | null = null;
  let blockCount = 0;

  const walk = (node: Node, depth: number): void => {
    if (rejection) return;
    if (node.nodeType === Node.TEXT_NODE) {
      out += node.textContent ?? '';
      return;
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return;
    const element = node as Element;
    const tag = element.tagName.toUpperCase();

    if (REJECTED_IN_CELL.has(tag)) {
      rejection = 'nested-block';
      return;
    }
    // A hard line break, or a second paragraph, is genuinely multi-line
    // content. A pipe-table row is one line, so this is refused rather than
    // flattened into a sentence the author did not write.
    if (tag === 'BR') {
      rejection = 'multiline-cell';
      return;
    }
    if (TRANSPARENT_WRAPPERS.has(tag) && depth > 0) {
      blockCount += 1;
      if (blockCount > 1 && (tag === 'P' || tag === 'DIV')) {
        rejection = 'multiline-cell';
        return;
      }
    }

    if (tag === 'A') {
      const inner = (element.textContent ?? '').replace(/\s+/g, ' ').trim();
      const markdown = inner ? linkMarkdown(element.getAttribute('href') ?? '', escapeCell(inner)) : null;
      out += markdown ?? inner;
      return;
    }
    if (tag === 'CODE') {
      const inner = escapeCell(element.textContent ?? '');
      out += inner ? '`' + inner.replace(/`/g, '') + '`' : '';
      return;
    }
    if (tag === 'STRONG' || tag === 'B') {
      const inner = escapeCell(element.textContent ?? '');
      out += inner ? `**${inner}**` : '';
      return;
    }
    if (tag === 'EM' || tag === 'I') {
      const inner = escapeCell(element.textContent ?? '');
      out += inner ? `*${inner}*` : '';
      return;
    }
    for (const child of Array.from(element.childNodes)) walk(child, depth + 1);
  };

  for (const child of Array.from(cell.childNodes)) walk(child, 1);
  if (rejection) return { reason: rejection };
  return { text: escapeCell(out) };
}

/**
 * Converts pasted clipboard HTML into a Markdown table.
 *
 * Parsing uses `DOMParser`, which builds an inert document: scripts do not run
 * and resources are not fetched. Only text and a small set of attributes are
 * ever read, and no HTML is re-emitted.
 */
export function htmlTableToMarkdown(html: string): TableConversion {
  if (typeof DOMParser === 'undefined' || !/<\s*table[\s>]/i.test(html)) {
    return { converted: false, reason: 'no-table' };
  }
  const doc = new DOMParser().parseFromString(html, 'text/html');
  // Only outermost tables count here. A nested table is a different problem
  // from two tables side by side, and it should be reported as the one it is —
  // so it falls through to the cell walker, which names it `nested-block`.
  const tables = Array.from(doc.querySelectorAll('table')).filter((t) => !t.parentElement?.closest('table'));
  if (tables.length === 0) return { converted: false, reason: 'no-table' };
  if (tables.length > 1) return { converted: false, reason: 'multiple-tables' };

  const table = tables[0];
  // Only a paste that *is* a table is converted. A table embedded in a page
  // excerpt keeps its surrounding content, and rewriting only part of a paste
  // would silently drop the rest.
  const outside = (doc.body.textContent ?? '').replace(table.textContent ?? '', '');
  if (outside.trim().length > 0) return { converted: false, reason: 'content-outside-table' };

  const rows: string[][] = [];
  let headerRowIndex = -1;
  const rowElements = Array.from(table.querySelectorAll('tr'));
  for (const row of rowElements) {
    const cells = Array.from(row.children).filter((c) => {
      const tag = c.tagName.toUpperCase();
      return tag === 'TD' || tag === 'TH';
    });
    if (cells.length === 0) continue;
    const rendered: string[] = [];
    for (const cell of cells) {
      const span = (cell.getAttribute('colspan') ?? '1').trim();
      const rowSpan = (cell.getAttribute('rowspan') ?? '1').trim();
      if ((span !== '' && span !== '1') || (rowSpan !== '' && rowSpan !== '1')) {
        return { converted: false, reason: 'merged-cells' };
      }
      const result = renderCell(cell);
      if ('reason' in result) return { converted: false, reason: result.reason };
      rendered.push(result.text);
    }
    if (headerRowIndex === -1 && cells.some((c) => c.tagName.toUpperCase() === 'TH')) {
      headerRowIndex = rows.length;
    }
    rows.push(rendered);
  }

  if (rows.length === 0) return { converted: false, reason: 'empty-table' };
  const columns = rows[0].length;
  if (columns === 0) return { converted: false, reason: 'empty-table' };
  // A pipe table is rectangular by construction. A ragged one would have to be
  // padded, which invents cells the author never wrote.
  if (rows.some((row) => row.length !== columns)) return { converted: false, reason: 'ragged-rows' };

  // A Markdown table must have a header row. When the source had no `<th>` the
  // first row becomes the header — that is what every renderer will do with the
  // result anyway, so it is the honest rendering rather than an invention.
  const header = rows[headerRowIndex === -1 ? 0 : headerRowIndex];
  const body = rows.filter((_, index) => index !== (headerRowIndex === -1 ? 0 : headerRowIndex));

  const lines = [
    `| ${header.join(' | ')} |`,
    `| ${header.map(() => '---').join(' | ')} |`,
    ...body.map((row) => `| ${row.join(' | ')} |`),
  ];
  return { converted: true, markdown: lines.join('\n'), rows: rows.length, columns };
}
