// Pane-width model for the four-pane workspace. Sidebar, search, and editor
// hold pixel widths; the preview takes the remaining space. Widths persist in
// localStorage under a versioned key; invalid stored values are ignored.

export const PANE_WIDTHS_KEY = 'notrios.paneWidths.v1';
export const SPLITTER_WIDTH = 6;

export interface PaneWidths {
  sidebar: number;
  search: number;
  editor: number;
}

export const DEFAULT_WIDTHS: PaneWidths = { sidebar: 220, search: 320, editor: 430 };

export const MIN_WIDTHS = { sidebar: 160, search: 240, editor: 280, preview: 260 } as const;

/** Minimum workspace width at which all four panes fit at their minimums. */
export const MIN_WORKSPACE_WIDTH =
  MIN_WIDTHS.sidebar + MIN_WIDTHS.search + MIN_WIDTHS.editor + MIN_WIDTHS.preview + 3 * SPLITTER_WIDTH;

function isUsable(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 && value < 100000;
}

/** sanitizeWidths accepts unknown parsed JSON and returns safe widths. */
export function sanitizeWidths(raw: unknown): PaneWidths {
  if (typeof raw !== 'object' || raw === null) return { ...DEFAULT_WIDTHS };
  const candidate = raw as Record<string, unknown>;
  const result = { ...DEFAULT_WIDTHS };
  if (isUsable(candidate.sidebar)) result.sidebar = candidate.sidebar;
  if (isUsable(candidate.search)) result.search = candidate.search;
  if (isUsable(candidate.editor)) result.editor = candidate.editor;
  return result;
}

export function loadPaneWidths(): PaneWidths {
  try {
    const raw = localStorage.getItem(PANE_WIDTHS_KEY);
    if (!raw) return { ...DEFAULT_WIDTHS };
    return sanitizeWidths(JSON.parse(raw));
  } catch {
    return { ...DEFAULT_WIDTHS };
  }
}

export function savePaneWidths(widths: PaneWidths) {
  try {
    localStorage.setItem(PANE_WIDTHS_KEY, JSON.stringify(widths));
  } catch {
    // Persistence is best-effort.
  }
}

/** previewWidth derives the remaining pane width for a container size. */
export function previewWidth(widths: PaneWidths, containerWidth: number): number {
  return containerWidth - 3 * SPLITTER_WIDTH - widths.sidebar - widths.search - widths.editor;
}

/**
 * clampWidths reconciles widths with the current container: every pane keeps
 * its minimum, nothing can be resized off-screen, and when the window shrinks
 * the panes give up space right-to-left (preview first, then editor, then
 * search, then sidebar) down to their minimums.
 */
export function clampWidths(widths: PaneWidths, containerWidth: number): PaneWidths {
  const w: PaneWidths = {
    sidebar: Math.max(MIN_WIDTHS.sidebar, widths.sidebar),
    search: Math.max(MIN_WIDTHS.search, widths.search),
    editor: Math.max(MIN_WIDTHS.editor, widths.editor),
  };
  if (containerWidth < MIN_WORKSPACE_WIDTH) {
    // Compact fallback: everything at its minimum; the workspace scrolls
    // horizontally below the documented minimum window width.
    return { sidebar: MIN_WIDTHS.sidebar, search: MIN_WIDTHS.search, editor: MIN_WIDTHS.editor };
  }
  let overflow = -previewWidth(w, containerWidth) + MIN_WIDTHS.preview;
  // overflow > 0 means the preview would fall below its minimum.
  for (const key of ['editor', 'search', 'sidebar'] as const) {
    if (overflow <= 0) break;
    const give = Math.min(overflow, w[key] - MIN_WIDTHS[key]);
    w[key] -= give;
    overflow -= give;
  }
  return w;
}

/**
 * equalizeEditorPreview splits the space left after the sidebar and search
 * panes equally between the editor and the preview. Applied when the window
 * (container) is resized: the editor and preview always come out the same
 * width after a resize, while splitter drags can still set individual sizes.
 */
export function equalizeEditorPreview(widths: PaneWidths, containerWidth: number): PaneWidths {
  const w = clampWidths(widths, containerWidth);
  const remaining = containerWidth - 3 * SPLITTER_WIDTH - w.sidebar - w.search;
  return clampWidths({ ...w, editor: remaining / 2 }, containerWidth);
}

export type SplitterIndex = 0 | 1 | 2;

/**
 * resizeAt moves the boundary after pane `index` by `delta` pixels (positive
 * = right). Splitter 0 trades sidebar/search, splitter 1 trades search/editor,
 * splitter 2 grows or shrinks the editor against the preview remainder.
 */
export function resizeAt(widths: PaneWidths, index: SplitterIndex, delta: number, containerWidth: number): PaneWidths {
  const w = { ...widths };
  if (index === 0) {
    const applied = clampDelta(delta, w.sidebar, MIN_WIDTHS.sidebar, w.search, MIN_WIDTHS.search);
    w.sidebar += applied;
    w.search -= applied;
  } else if (index === 1) {
    const applied = clampDelta(delta, w.search, MIN_WIDTHS.search, w.editor, MIN_WIDTHS.editor);
    w.search += applied;
    w.editor -= applied;
  } else {
    const preview = previewWidth(w, containerWidth);
    const applied = clampDelta(delta, w.editor, MIN_WIDTHS.editor, preview, MIN_WIDTHS.preview);
    w.editor += applied;
  }
  return clampWidths(w, containerWidth);
}

/** clampDelta limits a boundary move so neither neighbor goes below minimum. */
function clampDelta(delta: number, left: number, leftMin: number, right: number, rightMin: number): number {
  const maxGrow = Math.max(0, right - rightMin);
  const maxShrink = Math.max(0, left - leftMin);
  return Math.max(-maxShrink, Math.min(maxGrow, delta));
}
