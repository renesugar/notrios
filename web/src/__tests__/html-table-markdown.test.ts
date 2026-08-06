import { describe, expect, it, vi } from 'vitest';
import { htmlTableToMarkdown, type TableConversion } from '../html-table-markdown';
import { tablePasteHandler } from '../editor-extensions';

function convert(html: string): TableConversion {
  return htmlTableToMarkdown(html);
}

function markdownOf(html: string): string {
  const result = convert(html);
  if (!result.converted) throw new Error(`expected a conversion, got ${result.reason}`);
  return result.markdown;
}

function reasonOf(html: string): string {
  const result = convert(html);
  if (result.converted) throw new Error(`expected a refusal, got:\n${result.markdown}`);
  return result.reason;
}

describe('converting a simple table', () => {
  it('converts a header and body into a pipe table', () => {
    const markdown = markdownOf(`
      <table>
        <thead><tr><th>Item</th><th>Cost</th></tr></thead>
        <tbody><tr><td>Pan</td><td>12</td></tr><tr><td>Whisk</td><td>4</td></tr></tbody>
      </table>`);
    expect(markdown).toBe(
      '| Item | Cost |\n' +
      '| --- | --- |\n' +
      '| Pan | 12 |\n' +
      '| Whisk | 4 |',
    );
  });

  // Markdown has no headerless table. Promoting the first row is what every
  // renderer does with the result anyway, so it is the honest rendering rather
  // than an invention.
  it('promotes the first row when the source had no th', () => {
    const markdown = markdownOf('<table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>');
    expect(markdown).toBe('| a | b |\n| --- | --- |\n| c | d |');
  });

  it('keeps inline emphasis, code, and links', () => {
    const markdown = markdownOf(
      '<table><tr><th>What</th></tr><tr><td><strong>bold</strong> <em>it</em> <code>x</code></td></tr>' +
      '<tr><td><a href="https://example.com/x">site</a></td></tr></table>',
    );
    expect(markdown).toContain('| **bold** *it* `x` |');
    expect(markdown).toContain('| [site](https://example.com/x) |');
  });

  it('keeps an app URI link, which is what makes the graph see it', () => {
    const markdown = markdownOf(
      '<table><tr><td><a href="document://default/documents/doc_1">Kitchen</a></td></tr></table>',
    );
    expect(markdown).toContain('[Kitchen](document://default/documents/doc_1)');
  });

  // A pipe in cell text would otherwise open a new column.
  it('escapes pipes and backslashes and collapses whitespace', () => {
    const markdown = markdownOf('<table><tr><td>a | b</td><td>c\\d</td><td>  spaced\n  out  </td></tr></table>');
    expect(markdown.split('\n')[0]).toBe('| a \\| b | c\\\\d | spaced out |');
  });

  it('unwraps the wrapper elements real pastes are full of', () => {
    // Google Docs writes a <p> in every cell; spreadsheets add <span>/<font>.
    const markdown = markdownOf(
      '<table><tr><td><p><span>one</span></p></td><td><font color="red">two</font></td></tr></table>',
    );
    expect(markdown.split('\n')[0]).toBe('| one | two |');
  });

  it('ignores an empty trailing row rather than producing an empty line', () => {
    const markdown = markdownOf('<table><tr><td>a</td></tr><tr></tr></table>');
    expect(markdown).toBe('| a |\n| --- |');
  });
});

// The refusals are the point. A mangled table is worse than an HTML one,
// because the HTML at least renders.
describe('refusing what a pipe table cannot hold', () => {
  it('refuses merged cells', () => {
    expect(reasonOf('<table><tr><td colspan="2">wide</td></tr><tr><td>a</td><td>b</td></tr></table>'))
      .toBe('merged-cells');
    expect(reasonOf('<table><tr><td rowspan="2">tall</td><td>a</td></tr></table>'))
      .toBe('merged-cells');
  });

  it('refuses ragged rows rather than padding them', () => {
    expect(reasonOf('<table><tr><td>a</td><td>b</td></tr><tr><td>c</td></tr></table>')).toBe('ragged-rows');
  });

  it('refuses a nested table, list, or heading in a cell', () => {
    expect(reasonOf('<table><tr><td><table><tr><td>x</td></tr></table></td></tr></table>')).toBe('nested-block');
    expect(reasonOf('<table><tr><td><ul><li>x</li></ul></td></tr></table>')).toBe('nested-block');
    expect(reasonOf('<table><tr><td><h2>x</h2></td></tr></table>')).toBe('nested-block');
    expect(reasonOf('<table><tr><td><pre>x</pre></td></tr></table>')).toBe('nested-block');
  });

  // A pipe-table row is one line. Flattening a line break would put words on a
  // line the author never wrote.
  it('refuses a multi-line cell', () => {
    expect(reasonOf('<table><tr><td>one<br>two</td></tr></table>')).toBe('multiline-cell');
    expect(reasonOf('<table><tr><td><p>one</p><p>two</p></td></tr></table>')).toBe('multiline-cell');
  });

  it('refuses a paste that merely contains a table', () => {
    expect(reasonOf('<p>Some prose</p><table><tr><td>a</td></tr></table>')).toBe('content-outside-table');
    expect(reasonOf('<table><tr><td>a</td></tr></table><table><tr><td>b</td></tr></table>'))
      .toBe('multiple-tables');
  });

  it('does nothing for clipboard HTML with no table at all', () => {
    expect(reasonOf('<p>just text</p>')).toBe('no-table');
    expect(reasonOf('')).toBe('no-table');
  });

  it('refuses an empty table', () => {
    expect(reasonOf('<table></table>')).toBe('empty-table');
  });
});

describe('link targets that would not survive the syntax', () => {
  // A space ends an unquoted URL and an unescaped `)` closes the link early —
  // the same Markdown rules E1a and E3 each met in a fixture. Dropping the link
  // and keeping the text is recoverable; writing a broken link is less so.
  it('keeps the text and drops the link when the href would break', () => {
    expect(markdownOf('<table><tr><td><a href="/a b">text</a></td></tr></table>')).toContain('| text |');
    expect(markdownOf('<table><tr><td><a href="https://x/(y)">text</a></td></tr></table>')).toContain('| text |');
  });

  it('drops a javascript: target entirely', () => {
    const markdown = markdownOf('<table><tr><td><a href="javascript:alert(1)">click</a></td></tr></table>');
    expect(markdown).toContain('| click |');
    expect(markdown).not.toContain('javascript:');
  });
});

// Parsing is inert: DOMParser builds a document with no browsing context, so
// scripts do not run and resources are not fetched. Nothing is re-emitted as
// HTML either — only text and a whitelisted set of attributes are read.
describe('pasted markup is never executed or re-emitted', () => {
  it('drops script and event handlers instead of carrying them through', () => {
    const markdown = markdownOf(
      '<table><tr><td>safe<script>window.__pwned = 1</script></td>' +
      '<td><img src=x onerror="window.__pwned=2">alt</td></tr></table>',
    );
    expect(markdown).not.toContain('script');
    expect(markdown).not.toContain('onerror');
    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined();
  });
});

describe('the paste handler', () => {
  function pasteEvent(html: string | null) {
    return {
      clipboardData: { getData: (type: string) => (type === 'text/html' && html !== null ? html : '') },
      preventDefault: vi.fn(),
    } as unknown as ClipboardEvent & { preventDefault: ReturnType<typeof vi.fn> };
  }

  function fakeView() {
    const dispatched: unknown[] = [];
    return {
      dispatched,
      view: {
        state: { replaceSelection: (text: string) => ({ text }) },
        dispatch: (t: unknown) => dispatched.push(t),
      } as never,
    };
  }

  it('replaces the selection with the Markdown table and claims the paste', () => {
    const handler = tablePasteHandler();
    const { view, dispatched } = fakeView();
    const event = pasteEvent('<table><tr><th>a</th></tr><tr><td>b</td></tr></table>');
    expect(handler(event, view)).toBe(true);
    expect(event.preventDefault).toHaveBeenCalled();
    expect(dispatched).toEqual([{ text: '| a |\n| --- |\n| b |' }]);
  });

  // Falling through is what guarantees nothing a user pastes can be lost here.
  it('lets the ordinary paste happen for anything it refuses', () => {
    const handler = tablePasteHandler();
    for (const html of [
      null,
      '<p>prose</p>',
      '<table><tr><td colspan="2">merged</td></tr></table>',
      '<table><tr><td>one<br>two</td></tr></table>',
    ]) {
      const { view, dispatched } = fakeView();
      const event = pasteEvent(html);
      expect(handler(event, view)).toBe(false);
      expect(event.preventDefault).not.toHaveBeenCalled();
      expect(dispatched).toEqual([]);
    }
  });

  it('reports what it converted, for a caller that wants to say so', () => {
    const seen: Array<[number, number]> = [];
    const handler = tablePasteHandler((rows, columns) => seen.push([rows, columns]));
    const { view } = fakeView();
    handler(pasteEvent('<table><tr><th>a</th><th>b</th></tr><tr><td>c</td><td>d</td></tr></table>'), view);
    expect(seen).toEqual([[2, 2]]);
  });
});
