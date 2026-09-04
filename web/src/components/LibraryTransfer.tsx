// Import, export and snapshots — the operations that name a folder.
//
// These do not go through the REST API, because there is no REST route that
// starts them: the jobs API lists, reads, cancels, retries and resets, and its
// only start is the closed, path-free sync one. They go through the Wails
// bridge instead, which exists only when this window's own process owns the
// store. That is why the header button is greyed out in a browser rather than
// hidden: the capability is genuinely absent there, and a reader who cannot
// find a feature concludes it is missing, while one who finds it disabled with
// a reason learns something true.
import { useCallback, useEffect, useRef, useState } from 'react';
import { PublishPanel } from './PublishPanel';

/** The half of the native bridge this component uses. */
interface PublishProfile { name: string; description: string; target: string }

interface PublicationPlan {
  profile: string;
  target: string;
  manifest_sha256: string;
  counts: Record<string, number>;
  exclusions: Array<{ kind: string; id: string; reason: string }>;
  warnings: string[];
  truncated: boolean;
  /** The same run as a command line, digest included. */
  command: string;
}

interface PublicationResult {
  profile: string; directory: string; documents: number; objects: number; manifest_sha256: string;
}

interface TransferBridge {
  ChooseDirectory(purpose: string): Promise<string>;
  PublishProfiles?: () => Promise<PublishProfile[] | null>;
  PlanPublication?: (name: string) => Promise<PublicationPlan>;
  Publish?: (name: string, reviewedDigest: string, path: string) => Promise<PublicationResult>;
  ImportJoplinRaw(path: string, dryRun: boolean): Promise<TransferReport>;
  ImportObsidian(path: string, dryRun: boolean): Promise<TransferReport>;
  VerifyArchive(path: string): Promise<TransferReport>;
  ImportArchive(path: string): Promise<TransferReport>;
  ExportArchive(path: string): Promise<TransferReport>;
  CreateSnapshot(path: string): Promise<TransferReport>;
}

/**
 * Records what the interface did, where the desktop application can keep it.
 *
 * Deliberately never given a note title, a query, a tag or anything a person
 * wrote: what is recorded is which operation was asked for, against which
 * folder, and how it ended. In a browser there is nowhere to write it and this
 * does nothing, which is why every call ignores the result.
 */
function logAction(category: string, detail: string): void {
  const bound = (window as Window & {
    go?: { main?: { NativeUIBridge?: { LogAction?: (category: string, detail: string) => void } } };
  }).go?.main?.NativeUIBridge;
  try {
    bound?.LogAction?.(category, detail);
  } catch {
    // A transcript is never worth failing the operation it describes.
  }
}

interface TransferReport {
  kind: string;
  dry_run: boolean;
  job_id?: string;
  summary: unknown;
}

/**
 * Returns the native bridge, or undefined when this window does not own the
 * library: a browser, or a -gui-only window rendering a separate notriosd.
 * Undefined is the whole of the availability check; there is no flag to get out
 * of step with it.
 *
 * Note that a -gui-only window is normally pointed at 127.0.0.1, so this is not
 * about the service being far away. It is about the store belonging to another
 * program.
 */
export function transferBridge(): TransferBridge | undefined {
  const bound = (window as Window & { go?: { main?: { NativeUIBridge?: Partial<TransferBridge> } } })
    .go?.main?.NativeUIBridge;
  return bound?.ChooseDirectory ? (bound as TransferBridge) : undefined;
}

type OperationID = 'joplin' | 'obsidian' | 'archive' | 'export' | 'snapshot';

interface Operation {
  id: OperationID;
  title: string;
  /** What a person is choosing, said in their terms rather than the format's. */
  chooserLabel: string;
  /** The read-only step, where there is one worth offering separately. */
  previewLabel?: string;
  applyLabel: string;
  description: string;
  destructive?: string;
}

const OPERATIONS: Operation[] = [
  {
    id: 'joplin',
    title: 'Import from Joplin',
    chooserLabel: 'Joplin RAW export folder',
    previewLabel: 'Scan without importing',
    applyLabel: 'Import',
    description: 'Reads a Joplin RAW export. Scanning first tells you what is in it and writes nothing.',
  },
  {
    id: 'obsidian',
    title: 'Import from Obsidian',
    chooserLabel: 'Obsidian vault folder',
    previewLabel: 'Scan without importing',
    applyLabel: 'Import',
    description: 'Reads an Obsidian vault. Folder renames are a command-line option; this imports the vault as it stands.',
  },
  {
    id: 'archive',
    title: 'Import from Notrios',
    chooserLabel: 'Notrios archive folder',
    previewLabel: 'Verify the archive',
    applyLabel: 'Merge into this library',
    description: 'Reads an archive exported by Notrios. Verifying opens no database and changes nothing; merging keeps this library’s identity and admits the archive’s notes alongside your own.',
    destructive: 'Replacing, forking and adopting a library are command-line operations. This one only adds.',
  },
  {
    id: 'export',
    title: 'Export this library',
    chooserLabel: 'Folder to write the archive into',
    applyLabel: 'Export everything',
    description: 'Writes a complete archive. Exporting part of a library is a command-line option, because choosing a subset means seeing what it selects first.',
  },
  {
    id: 'snapshot',
    title: 'Take a snapshot',
    chooserLabel: 'Folder to write the snapshot into',
    applyLabel: 'Create snapshot',
    description: 'Writes a verified snapshot image. A snapshot that fails verification is never recorded as one.',
  },
];

function summarise(report: TransferReport): string {
  const summary = report.summary;
  if (summary && typeof summary === 'object') {
    const parts = Object.entries(summary as Record<string, unknown>)
      .filter(([, value]) => typeof value === 'number' || typeof value === 'boolean' || typeof value === 'string')
      .slice(0, 6)
      .map(([key, value]) => `${key.replaceAll('_', ' ')}: ${String(value)}`);
    if (parts.length > 0) return parts.join(' · ');
  }
  return report.dry_run ? 'Nothing was written.' : 'Finished.';
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function LibraryTransfer({ onClose }: { onClose: () => void }) {
  const bridge = transferBridge();
  const [paths, setPaths] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState('');
  const [reports, setReports] = useState<Record<string, string>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const closeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    closeRef.current?.focus();
    logAction('transfer', 'the import and export dialog opened');
  }, []);
  // Escape closes, which is what a person expects of any dialog and what this
  // one previously did not do. Bound on the document because focus may be
  // anywhere inside by the time it is pressed.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  const run = useCallback(async (operation: OperationID, key: string, work: (path: string) => Promise<TransferReport>) => {
    const path = (paths[operation] ?? '').trim();
    setBusy(key);
    setErrors((current) => ({ ...current, [operation]: '' }));
    logAction('transfer', `${key} requested for ${path}`);
    const started = Date.now();
    try {
      const report = await work(path);
      setReports((current) => ({ ...current, [operation]: summarise(report) }));
      logAction('transfer', `${key} finished in ${Date.now() - started}ms — ${summarise(report)}`);
    } catch (error) {
      setErrors((current) => ({ ...current, [operation]: errorMessage(error) }));
      logAction('transfer', `${key} failed after ${Date.now() - started}ms — ${errorMessage(error)}`);
    } finally {
      setBusy('');
    }
  }, [paths]);

  const choose = useCallback(async (operation: OperationID) => {
    if (!bridge) return;
    const chosen = await bridge.ChooseDirectory(operation === 'archive' ? 'export' : operation);
    logAction('transfer', chosen ? `chose ${chosen} for ${operation}` : `cancelled the chooser for ${operation}`);
    if (chosen) setPaths((current) => ({ ...current, [operation]: chosen }));
  }, [bridge]);

  const preview = (operation: OperationID) => {
    if (!bridge) return;
    if (operation === 'joplin') return run(operation, `${operation}-preview`, (path) => bridge.ImportJoplinRaw(path, true));
    if (operation === 'obsidian') return run(operation, `${operation}-preview`, (path) => bridge.ImportObsidian(path, true));
    if (operation === 'archive') return run(operation, `${operation}-preview`, (path) => bridge.VerifyArchive(path));
  };

  const apply = (operation: OperationID) => {
    if (!bridge) return;
    const work: Record<OperationID, (path: string) => Promise<TransferReport>> = {
      joplin: (path) => bridge.ImportJoplinRaw(path, false),
      obsidian: (path) => bridge.ImportObsidian(path, false),
      archive: (path) => bridge.ImportArchive(path),
      export: (path) => bridge.ExportArchive(path),
      snapshot: (path) => bridge.CreateSnapshot(path),
    };
    return run(operation, `${operation}-apply`, work[operation]);
  };

  return (
    <div className="sync-center-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      {/* sync-center-single because this dialog has no section rail; without it
          the body is laid into the 190px tab column. */}
      <section className="sync-center sync-center-single" role="dialog" aria-modal="true"
        aria-labelledby="library-transfer-title" data-testid="library-transfer">
        <header className="sync-center-header">
          <div>
            <p className="eyebrow">This machine</p>
            <h2 id="library-transfer-title">Import and export</h2>
            <p className="muted">Every operation here names a folder on the computer running this library.</p>
          </div>
          <button ref={closeRef} type="button" className="icon-button sync-close" aria-label="Close import and export"
            onClick={onClose}>×</button>
        </header>

        <div className="sync-center-body">
          {OPERATIONS.map((operation) => (
            <div key={operation.id} className="sync-section" data-testid={`transfer-${operation.id}`}>
              <h3>{operation.title}</h3>
              <p className="muted">{operation.description}</p>
              {operation.destructive ? <p className="boundary-note">{operation.destructive}</p> : null}
              <label>{operation.chooserLabel}
                <span className="directory-picker">
                  <input
                    value={paths[operation.id] ?? ''}
                    data-testid={`transfer-path-${operation.id}`}
                    onChange={(event) => setPaths((current) => ({ ...current, [operation.id]: event.target.value }))}
                    placeholder="/absolute/path/to/folder"
                    autoComplete="off"
                  />
                  <button type="button" data-testid={`transfer-choose-${operation.id}`} disabled={busy !== ''}
                    onClick={() => void choose(operation.id)}>Choose folder…</button>
                </span>
              </label>
              <div className="row-actions">
                {operation.previewLabel ? (
                  <button type="button" data-testid={`transfer-preview-${operation.id}`}
                    disabled={busy !== '' || !(paths[operation.id] ?? '').trim()}
                    onClick={() => void preview(operation.id)}>{operation.previewLabel}</button>
                ) : null}
                <button type="button" className="primary-button" data-testid={`transfer-apply-${operation.id}`}
                  disabled={busy !== '' || !(paths[operation.id] ?? '').trim()}
                  onClick={() => void apply(operation.id)}>{operation.applyLabel}</button>
              </div>
              {reports[operation.id] ? (
                <p className="success-line" data-testid={`transfer-report-${operation.id}`}>{reports[operation.id]}</p>
              ) : null}
              {errors[operation.id] ? (
                <p className="sync-inline-notice error" role="status"
                  data-testid={`transfer-error-${operation.id}`}>{errors[operation.id]}</p>
              ) : null}
            </div>
          ))}
          {/* Publishing sits with the other operations that move data out of
              the library, but it is the only one that rewrites what it emits,
              so it carries its own review rather than a dry run. */}
          <PublishPanel bridge={bridge} />
        </div>
      </section>
    </div>
  );
}
