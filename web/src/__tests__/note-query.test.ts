import { describe, expect, it, vi } from 'vitest';
import { normalizePreviewHTML } from '../preview-utils';
import { renderNoteQueryBlocks, renderNoteQueryResult } from '../note-query';
import type { NoteQueryResult } from '../api';

function blockElement(source = 'query: tag:todo'): HTMLElement {
  const node = document.createElement('div');
  node.setAttribute('data-note-query', '');
  node.setAttribute('data-note-query-source', source);
  return node;
}

function result(overrides: Partial<NoteQueryResult> = {}): NoteQueryResult {
  return {
    spec: { query: 'tag:todo', fields: ['title'], sort: 'updated', limit: 10 },
    rows: [],
    truncated: false,
    ...overrides,
  };
}

describe('turning a fence into a placeholder', () => {
  it('replaces a note-query fence with a placeholder carrying its source', () => {
    const html = normalizePreviewHTML(
      '<pre><code class="language-note-query">query: tag:todo\nlimit: 5</code></pre>',
    );
    const host = document.createElement('div');
    host.innerHTML = html;
    const block = host.querySelector('[data-note-query]');
    expect(block).toBeTruthy();
    expect(block?.getAttribute('data-note-query-source')).toBe('query: tag:todo\nlimit: 5');
    expect(host.querySelector('pre')).toBeNull();
    // Until results arrive the block says it is working, not that it is empty.
    expect(host.textContent).toContain('Running query…');
  });

  it('leaves ordinary code fences alone', () => {
    const html = normalizePreviewHTML('<pre><code class="language-python">print(1)</code></pre>');
    const host = document.createElement('div');
    host.innerHTML = html;
    expect(host.querySelector('[data-note-query]')).toBeNull();
    expect(host.querySelector('pre')).toBeTruthy();
  });

  // The source travels as an attribute, so the serializer escapes it and no
  // note content is ever concatenated into markup.
  it('does not let block text escape into markup', () => {
    const html = normalizePreviewHTML(
      '<pre><code class="language-note-query">query: "&lt;img src=x onerror=alert(1)&gt;"</code></pre>',
    );
    const host = document.createElement('div');
    host.innerHTML = html;
    expect(host.querySelector('img')).toBeNull();
    expect(host.querySelector('[data-note-query]')?.getAttribute('data-note-query-source'))
      .toContain('<img src=x onerror=alert(1)>');
  });
});

describe('rendering a result', () => {
  it('lists matching notes as routed links', () => {
    const block = blockElement();
    renderNoteQueryResult(block, result({
      rows: [{ document_id: 'doc_1', uri: 'document://default/documents/doc_1', title: 'Sourdough' }],
    }));
    const link = block.querySelector('a');
    expect(link?.textContent).toBe('Sourdough');
    expect(link?.getAttribute('data-app-uri')).toBe('document://default/documents/doc_1');
    expect(link?.getAttribute('href')).toBe('#');
  });

  it('shows only the fields the block asked for', () => {
    const row = {
      document_id: 'doc_1', uri: 'document://default/documents/doc_1', title: 'Sourdough',
      notebook: 'Recipes', tags: ['todo'], updated_at: '2026-01-02T00:00:00Z', snippet: 'flour water',
    };
    const bare = blockElement();
    renderNoteQueryResult(bare, result({ rows: [row] }));
    expect(bare.textContent).not.toContain('Recipes');
    expect(bare.textContent).not.toContain('flour water');

    const full = blockElement();
    renderNoteQueryResult(full, result({
      rows: [row],
      spec: { query: 'x', fields: ['title', 'notebook', 'tags', 'updated', 'snippet'], sort: 'updated', limit: 10 },
    }));
    expect(full.textContent).toContain('Recipes');
    expect(full.textContent).toContain('#todo');
    expect(full.textContent).toContain('flour water');
  });

  // A list that silently stops is a list that lies about the library.
  it('always says when results were truncated', () => {
    const block = blockElement();
    renderNoteQueryResult(block, result({
      rows: [{ document_id: 'd', uri: 'u', title: 't' }],
      truncated: true,
    }));
    expect(block.textContent).toContain('more notes match');
  });

  it('says nothing matched rather than rendering an empty box', () => {
    const block = blockElement();
    renderNoteQueryResult(block, result());
    expect(block.textContent).toContain('No notes match');
  });

  it('renders an error inside the block', () => {
    const block = blockElement();
    renderNoteQueryResult(block, result({ error: 'unknown key "exec"' }));
    expect(block.textContent).toContain('unknown key');
    expect(block.getAttribute('data-note-query-state')).toBe('error');
  });

  // Note content is untrusted, so every value is written as text.
  it('writes result values as text, never as markup', () => {
    const block = blockElement();
    renderNoteQueryResult(block, result({
      rows: [{ document_id: 'd', uri: 'u', title: '<img src=x onerror="window.__pwned=1">' }],
    }));
    expect(block.querySelector('img')).toBeNull();
    expect(block.textContent).toContain('<img src=x');
    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined();
  });
});

describe('evaluating the blocks in a note', () => {
  it('fills each block and leaves the rest of the note alone', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      JSON.stringify(result({ rows: [{ document_id: 'd', uri: 'u', title: 'Focaccia' }] })),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )));
    const container = document.createElement('div');
    container.innerHTML = '<p>before</p>';
    container.append(blockElement());
    container.append(document.createElement('hr'));

    renderNoteQueryBlocks(container);
    await new Promise((resolve) => setTimeout(resolve, 30));
    expect(container.textContent).toContain('Focaccia');
    expect(container.textContent).toContain('before');
    vi.unstubAllGlobals();
  });

  // A block that cannot reach the service says so and the note is unaffected.
  it('reports an unreachable service inside the block', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new Error('offline'); }));
    const container = document.createElement('div');
    const block = blockElement();
    container.append(block);

    renderNoteQueryBlocks(container);
    await new Promise((resolve) => setTimeout(resolve, 30));
    expect(block.textContent).toContain('could not be reached');
    vi.unstubAllGlobals();
  });

  it('aborts in flight work so a late reply cannot write into another note', async () => {
    const abortSignals: Array<AbortSignal | undefined> = [];
    vi.stubGlobal('fetch', vi.fn(async (_url: string, init?: RequestInit) => {
      abortSignals.push(init?.signal ?? undefined);
      return new Promise(() => {}) as Promise<Response>;
    }));
    const container = document.createElement('div');
    container.append(blockElement());
    const cleanup = renderNoteQueryBlocks(container);
    await new Promise((resolve) => setTimeout(resolve, 10));
    cleanup();
    expect(abortSignals[0]?.aborted).toBe(true);
    vi.unstubAllGlobals();
  });
});
