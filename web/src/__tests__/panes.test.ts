// Pane-width model: minimums, clamping on shrink, boundary drags, and safe
// handling of invalid persisted values.
import { describe, expect, it } from 'vitest';
import {
  clampWidths,
  DEFAULT_WIDTHS,
  equalizeEditorPreview,
  MIN_WIDTHS,
  MIN_WORKSPACE_WIDTH,
  previewWidth,
  resizeAt,
  sanitizeWidths,
} from '../panes';

const WIDE = 1400;

describe('sanitizeWidths', () => {
  it('ignores invalid, negative, nonnumeric, and absurd stored values', () => {
    expect(sanitizeWidths(null)).toEqual(DEFAULT_WIDTHS);
    expect(sanitizeWidths('nonsense')).toEqual(DEFAULT_WIDTHS);
    expect(sanitizeWidths({ sidebar: -5, search: 'wide', editor: Infinity })).toEqual(DEFAULT_WIDTHS);
    expect(sanitizeWidths({ sidebar: 1e9 })).toEqual(DEFAULT_WIDTHS);
    expect(sanitizeWidths({ sidebar: 300 })).toEqual({ ...DEFAULT_WIDTHS, sidebar: 300 });
  });
});

describe('clampWidths', () => {
  it('enforces every pane minimum including the preview remainder', () => {
    const clamped = clampWidths({ sidebar: 10, search: 10, editor: 10 }, WIDE);
    expect(clamped.sidebar).toBeGreaterThanOrEqual(MIN_WIDTHS.sidebar);
    expect(clamped.search).toBeGreaterThanOrEqual(MIN_WIDTHS.search);
    expect(clamped.editor).toBeGreaterThanOrEqual(MIN_WIDTHS.editor);
    expect(previewWidth(clamped, WIDE)).toBeGreaterThanOrEqual(MIN_WIDTHS.preview);
  });

  it('reconciles oversized stored widths when the window shrinks', () => {
    const clamped = clampWidths({ sidebar: 600, search: 600, editor: 600 }, 1200);
    expect(previewWidth(clamped, 1200)).toBeGreaterThanOrEqual(MIN_WIDTHS.preview);
    expect(clamped.sidebar + clamped.search + clamped.editor).toBeLessThan(1200);
  });

  it('falls back to all minimums below the documented minimum workspace width', () => {
    const clamped = clampWidths(DEFAULT_WIDTHS, MIN_WORKSPACE_WIDTH - 100);
    expect(clamped).toEqual({ sidebar: MIN_WIDTHS.sidebar, search: MIN_WIDTHS.search, editor: MIN_WIDTHS.editor });
  });
});

describe('equalizeEditorPreview', () => {
  it('gives the editor and preview equal widths after a window resize', () => {
    for (const container of [1400, 1200, 1600]) {
      const next = equalizeEditorPreview({ sidebar: 220, search: 320, editor: 500 }, container);
      expect(next.sidebar).toBe(220);
      expect(next.search).toBe(320);
      expect(next.editor).toBe(previewWidth(next, container));
    }
  });

  it('keeps every pane at or above its minimum on narrow windows', () => {
    const next = equalizeEditorPreview(DEFAULT_WIDTHS, MIN_WORKSPACE_WIDTH + 20);
    expect(next.editor).toBeGreaterThanOrEqual(MIN_WIDTHS.editor);
    expect(previewWidth(next, MIN_WORKSPACE_WIDTH + 20)).toBeGreaterThanOrEqual(MIN_WIDTHS.preview);
  });

  it('falls back to minimums below the minimum workspace width', () => {
    const next = equalizeEditorPreview(DEFAULT_WIDTHS, MIN_WORKSPACE_WIDTH - 50);
    expect(next).toEqual({ sidebar: MIN_WIDTHS.sidebar, search: MIN_WIDTHS.search, editor: MIN_WIDTHS.editor });
  });
});

describe('resizeAt', () => {
  it('splitter 0 trades width between sidebar and search only', () => {
    const next = resizeAt(DEFAULT_WIDTHS, 0, 40, WIDE);
    expect(next.sidebar).toBe(DEFAULT_WIDTHS.sidebar + 40);
    expect(next.search).toBe(DEFAULT_WIDTHS.search - 40);
    expect(next.editor).toBe(DEFAULT_WIDTHS.editor);
  });

  it('splitter 1 trades width between search and editor', () => {
    const next = resizeAt(DEFAULT_WIDTHS, 1, -30, WIDE);
    expect(next.search).toBe(DEFAULT_WIDTHS.search - 30);
    expect(next.editor).toBe(DEFAULT_WIDTHS.editor + 30);
  });

  it('splitter 2 grows the editor against the preview remainder', () => {
    const before = previewWidth(DEFAULT_WIDTHS, WIDE);
    const next = resizeAt(DEFAULT_WIDTHS, 2, 50, WIDE);
    expect(next.editor).toBe(DEFAULT_WIDTHS.editor + 50);
    expect(previewWidth(next, WIDE)).toBe(before - 50);
  });

  it('cannot push any pane below its minimum or off-screen', () => {
    const crushedLeft = resizeAt(DEFAULT_WIDTHS, 0, -10000, WIDE);
    expect(crushedLeft.sidebar).toBe(MIN_WIDTHS.sidebar);
    const crushedRight = resizeAt(DEFAULT_WIDTHS, 2, 10000, WIDE);
    expect(previewWidth(crushedRight, WIDE)).toBeGreaterThanOrEqual(MIN_WIDTHS.preview);
    const crushedSearch = resizeAt(DEFAULT_WIDTHS, 1, 10000, WIDE);
    expect(crushedSearch.editor).toBeGreaterThanOrEqual(MIN_WIDTHS.editor);
  });
});
