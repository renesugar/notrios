// Workspace layout invariants: DOM order of the four panes, three keyboard-
// accessible separators, no legacy centered max-width, keyboard resizing,
// pointer dragging, and safe handling of invalid persisted widths.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from '../App';
import { DEFAULT_WIDTHS, MIN_WIDTHS, PANE_WIDTHS_KEY } from '../panes';

// The real editor is heavyweight and jsdom-hostile; substitute inert stubs
// that preserve the value/onChange/readOnly contract.
vi.mock('md-editor-rt', () => ({
  MdEditor: ({ value, readOnly }: { value: string; readOnly?: boolean }) => (
    <textarea data-testid="editor-stub" readOnly={readOnly} value={value} onChange={() => {}} />
  ),
  MdPreview: ({ value }: { value: string }) => <div data-testid="preview-stub">{value}</div>,
}));
vi.mock('md-editor-rt/lib/style.css', () => ({}));

vi.mock('../api', async (importOriginal) => {
  const original = await importOriginal<typeof import('../api')>();
  return {
    ...original,
    getStatus: vi.fn().mockResolvedValue({
      service: 'notrios',
      version: 'test',
      status: 'running',
      search_sidecar: {
        configured: true,
        available: true,
        active: true,
        state: 'active',
        backlog: 2,
        failed_jobs: 0,
        last_sync_at: '2026-07-27T01:02:03Z',
      },
    }),
    getNotebookTree: vi.fn().mockResolvedValue([
      { id: 'nb_help', name: 'Help', builtin: true, position: 0 },
      { id: 'nb_notes', name: 'Notes', builtin: false, position: 0 },
    ]),
    listSearchNotebooks: vi.fn().mockResolvedValue([
      { id: 'snb_trash', name: 'Trash', query: 'is:trashed', builtin: true, sort_anchor: 'last' },
      { id: 'snb_all_notes', name: 'All notes', query: '', builtin: true, sort_anchor: 'first' },
    ]),
    listTags: vi.fn().mockResolvedValue([{ id: 'tag_1', name: 'todo', note_count: 2 }]),
    search: vi.fn().mockResolvedValue({ hits: [], next_cursor: '' }),
  };
});

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('workspace layout', () => {
  it('shows live sidecar state and backlog in the header', async () => {
    render(<App />);
    expect(await screen.findByText(/Recoll active · 2 pending · synced/)).toBeInTheDocument();
  });

  it('advertises the bounded boolean query syntax', async () => {
    render(<App />);
    const input = await screen.findByRole('textbox', { name: 'Search query' });
    expect(input).toHaveAttribute('maxlength', '4096');
    expect(screen.getByText(/Uppercase OR · implicit AND/)).toBeInTheDocument();
  });

  it('renders the four panes in DOM order with three separators between them', async () => {
    render(<App />);
    const workspace = await screen.findByTestId('workspace');
    const children = Array.from(workspace.children).filter((el) => el.tagName !== 'STYLE');
    const kinds = children.map((el) => el.getAttribute('data-testid'));
    expect(kinds).toEqual([
      'pane-sidebar',
      'pane-splitter',
      'pane-search',
      'pane-splitter',
      'pane-editor',
      'pane-splitter',
      'pane-preview',
    ]);
    const separators = screen.getAllByRole('separator');
    expect(separators).toHaveLength(3);
    for (const separator of separators) {
      expect(separator).toHaveAttribute('aria-orientation', 'vertical');
      expect(separator).toHaveAttribute('tabindex', '0');
      expect(separator.getAttribute('aria-valuenow')).toBeTruthy();
      expect(separator.getAttribute('aria-valuemin')).toBeTruthy();
      expect(separator.getAttribute('aria-valuemax')).toBeTruthy();
    }
  });

  it('does not use the legacy centered dashboard width', async () => {
    render(<App />);
    await screen.findByTestId('workspace');
    // The old layout constrained .app-shell to max-width: 1180px.
    expect(document.querySelector('.app-shell')).not.toHaveStyle({ maxWidth: '1180px' });
    const css = Array.from(document.querySelectorAll('style'))
      .map((el) => el.textContent ?? '')
      .join('\n');
    expect(css).not.toContain('1180px');
  });

  it('supports keyboard resizing with Arrow and Shift+Arrow steps', async () => {
    render(<App />);
    await screen.findByTestId('workspace');
    const separator = screen.getAllByRole('separator')[0];
    const before = Number(separator.getAttribute('aria-valuenow'));
    fireEvent.keyDown(separator, { key: 'ArrowRight' });
    await waitFor(() => expect(Number(separator.getAttribute('aria-valuenow'))).toBe(before + 16));
    fireEvent.keyDown(separator, { key: 'ArrowLeft', shiftKey: true });
    await waitFor(() => expect(Number(separator.getAttribute('aria-valuenow'))).toBe(before + 16 - 64));
  });

  it('resizes the adjacent pane during pointer drags and stops on release', async () => {
    render(<App />);
    await screen.findByTestId('workspace');
    const separator = screen.getAllByRole('separator')[0];
    const before = Number(separator.getAttribute('aria-valuenow'));

    fireEvent.pointerDown(separator, { pointerId: 1, clientX: 300, button: 0 });
    fireEvent.pointerMove(separator, { pointerId: 1, clientX: 340 });
    // Drag updates are coalesced through requestAnimationFrame.
    await waitFor(() => expect(Number(separator.getAttribute('aria-valuenow'))).toBe(before + 40));
    fireEvent.pointerUp(separator, { pointerId: 1 });
    expect(document.body.classList.contains('splitter-dragging')).toBe(false);

    // Movement after release must not resize anything.
    fireEvent.pointerMove(separator, { pointerId: 1, clientX: 500 });
    await new Promise((resolve) => setTimeout(resolve, 30));
    expect(Number(separator.getAttribute('aria-valuenow'))).toBe(before + 40);
  });

  it('ignores invalid persisted widths and clamps to minimums', async () => {
    localStorage.setItem(PANE_WIDTHS_KEY, JSON.stringify({ sidebar: -400, search: 'huge', editor: null }));
    render(<App />);
    await screen.findByTestId('workspace');
    const separator = screen.getAllByRole('separator')[0];
    const value = Number(separator.getAttribute('aria-valuenow'));
    expect(value).toBeGreaterThanOrEqual(MIN_WIDTHS.sidebar);
    expect(value).toBe(DEFAULT_WIDTHS.sidebar);
  });

  it('re-splits editor and preview equally when the window is resized', async () => {
    // Controllable ResizeObserver so the test can simulate a window resize.
    class ControlledResizeObserver {
      static instances: ControlledResizeObserver[] = [];
      constructor(private callback: ResizeObserverCallback) {
        ControlledResizeObserver.instances.push(this);
      }
      observe() {}
      unobserve() {}
      disconnect() {}
      trigger() {
        this.callback([], this as unknown as ResizeObserver);
      }
    }
    const originalObserver = globalThis.ResizeObserver;
    globalThis.ResizeObserver = ControlledResizeObserver as unknown as typeof ResizeObserver;
    try {
      render(<App />);
      const workspace = await screen.findByTestId('workspace');
      // Initial measurement saw 1400 (the setup.ts clientWidth); shrink the
      // container and notify the observer, as a real window resize would.
      Object.defineProperty(workspace, 'clientWidth', { configurable: true, get: () => 1240 });
      ControlledResizeObserver.instances.forEach((instance) => instance.trigger());

      // 1240 - 3 splitters (18) - sidebar 220 - search 320 = 682 → 341 each.
      const editorSeparator = screen.getAllByRole('separator')[2];
      await waitFor(() => expect(Number(editorSeparator.getAttribute('aria-valuenow'))).toBe(341));
      const css = Array.from(document.querySelectorAll('style'))
        .map((el) => el.textContent ?? '')
        .join('\n');
      expect(css).toContain('.editor-pane{width:341px}');
      expect(css).toContain('.preview-pane{width:341px}');
    } finally {
      globalThis.ResizeObserver = originalObserver;
    }
  });

  it('composes the sidebar with All notes first, Help immediately above Trash, tags below', async () => {
    render(<App />);
    const sidebar = await screen.findByTestId('pane-sidebar');
    await screen.findByTestId('sidebar-row-snb_all_notes');
    const buttons = Array.from(sidebar.querySelectorAll('[data-testid^="sidebar-row-"]')).map((el) =>
      el.getAttribute('data-testid'),
    );
    expect(buttons[0]).toBe('sidebar-row-snb_all_notes');
    expect(buttons.at(-1)).toBe('sidebar-row-snb_trash');
    expect(buttons.at(-2)).toBe('sidebar-row-nb_help');
    // Tags render in their own list after the notebook list.
    const tagList = await screen.findByTestId('sidebar-tags');
    expect(tagList.textContent).toContain('todo');
  });
});
