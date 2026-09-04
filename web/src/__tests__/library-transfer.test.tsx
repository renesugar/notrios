// The import/export surface, and the one thing about it a reader cannot check
// by eye: that it is disabled rather than missing when there is no bridge.
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from '../App';
import { LibraryTransfer } from '../components/LibraryTransfer';

// The real editor is jsdom-hostile -- it builds a CodeMirror state at mount and
// throws on a duplicated @codemirror/state -- and the header is what these two
// App cases are about, so it is stubbed exactly as the layout tests stub it.
vi.mock('md-editor-rt', () => ({
  MdEditor: ({ value, readOnly, defToolbars }: { value: string; readOnly?: boolean; defToolbars?: React.ReactNode }) => (
    <>
      <div data-testid="editor-toolbar-stub">{defToolbars}</div>
      <textarea data-testid="editor-stub" readOnly={readOnly} value={value} onChange={() => {}} />
    </>
  ),
  MdPreview: ({ value }: { value: string }) => <div data-testid="preview-stub">{value}</div>,
  config: () => {},
  DropdownToolbar: ({ children, overlay }: { children?: React.ReactNode; overlay?: React.ReactNode }) => (
    <div data-testid="dropdown-toolbar">{children}{overlay}</div>
  ),
  allToolbar: [],
}));
vi.mock('md-editor-rt/lib/style.css', () => ({}));

function bindBridge(overrides: Record<string, unknown> = {}) {
  const bridge = {
    ChooseDirectory: vi.fn(async () => '/tmp/chosen'),
    ImportJoplinRaw: vi.fn(async () => ({ kind: 'import_joplin_raw', dry_run: true, summary: { notes: 3 } })),
    ImportObsidian: vi.fn(async () => ({ kind: 'import_obsidian', dry_run: true, summary: {} })),
    VerifyArchive: vi.fn(async () => ({ kind: 'verify_archive_v2', dry_run: true, summary: { objects: 9 } })),
    ImportArchive: vi.fn(async () => ({ kind: 'import_archive_v2', dry_run: false, summary: {} })),
    ExportArchive: vi.fn(async () => ({ kind: 'export_archive_v2', dry_run: false, summary: {} })),
    CreateSnapshot: vi.fn(async () => ({ kind: 'snapshot_image', dry_run: false, summary: {} })),
    ...overrides,
  };
  Object.defineProperty(window, 'go', { configurable: true, value: { main: { NativeUIBridge: bridge } } });
  return bridge;
}

afterEach(() => Reflect.deleteProperty(window, 'go'));

describe('import and export', () => {
  it('offers importing from Notrios alongside the foreign formats', () => {
    bindBridge();
    render(<LibraryTransfer onClose={() => undefined} />);
    expect(screen.getByText('Import from Joplin')).toBeInTheDocument();
    expect(screen.getByText('Import from Obsidian')).toBeInTheDocument();
    expect(screen.getByText('Import from Notrios')).toBeInTheDocument();
    expect(screen.getByText('Export this library')).toBeInTheDocument();
    expect(screen.getByText('Take a snapshot')).toBeInTheDocument();
  });

  it('verifies a Notrios archive without merging it', async () => {
    const bridge = bindBridge();
    render(<LibraryTransfer onClose={() => undefined} />);
    await userEvent.type(screen.getByTestId('transfer-path-archive'), '/tmp/archive');
    await userEvent.click(screen.getByTestId('transfer-preview-archive'));
    expect(bridge.VerifyArchive).toHaveBeenCalledExactlyOnceWith('/tmp/archive');
    expect(bridge.ImportArchive).not.toHaveBeenCalled();
    expect(await screen.findByTestId('transfer-report-archive')).toHaveTextContent('objects: 9');
  });

  it('scans a Joplin export without importing it', async () => {
    const bridge = bindBridge();
    render(<LibraryTransfer onClose={() => undefined} />);
    await userEvent.type(screen.getByTestId('transfer-path-joplin'), '/tmp/joplin');
    await userEvent.click(screen.getByTestId('transfer-preview-joplin'));
    // The dry-run flag is the whole difference between looking and writing.
    expect(bridge.ImportJoplinRaw).toHaveBeenCalledExactlyOnceWith('/tmp/joplin', true);
  });

  it('reports a failure against the operation that failed', async () => {
    bindBridge({ ExportArchive: vi.fn(async () => { throw new Error('no such folder'); }) });
    render(<LibraryTransfer onClose={() => undefined} />);
    await userEvent.type(screen.getByTestId('transfer-path-export'), '/tmp/nowhere');
    await userEvent.click(screen.getByTestId('transfer-apply-export'));
    expect(await screen.findByTestId('transfer-error-export')).toHaveTextContent('no such folder');
  });

  it('greys the header control out in a browser rather than hiding it', () => {
    render(<App />);
    const button = screen.getByTestId('transfer-header-button');
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('title', expect.stringContaining('desktop app'));
  });

  it('enables the header control when the desktop bridge is bound', () => {
    bindBridge();
    render(<App />);
    expect(screen.getByTestId('transfer-header-button')).toBeEnabled();
  });
});
