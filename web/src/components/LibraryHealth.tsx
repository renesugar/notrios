// What has rotted in this library, and what could be reclaimed.
//
// Both reports here are read-only, and that is the service's decision rather
// than an omission this screen is working around. The lint endpoint has no
// apply at all; the garbage collector says so in its own description —
// "intentionally no REST apply endpoint; use notriosctl gc --apply for explicit
// local deletion". So this shows what is wrong and names the command that fixes
// it, rather than routing around a deliberate boundary through the desktop
// bridge the way import and export legitimately do.
//
// The difference is worth stating because the two cases look alike from here.
// Import has no REST route because a path is not something the API accepts;
// nothing forbids the operation. Fixing and collecting have no route because
// somebody decided they should be explicit and local. Building a button for the
// first is completing the interface; building one for the second is overruling
// a decision.
import { useCallback, useEffect, useState } from 'react';
import { getGarbageReport, getLintReport, type GarbageCollectionReport, type LintReport } from '../api';

interface FixEdit { kind: string; line: number }
interface FixDocumentPlan { document_id: string; base_revision_id: string; edits: FixEdit[] }
interface FixPlan {
  documents: FixDocumentPlan[];
  total_edits: number;
  total_documents: number;
  truncated: boolean;
  warnings: string[];
}
interface FixApplyResult { document_id: string; applied: number; skipped: number; error?: string }
interface FixPlanReport { plan: FixPlan; applied: boolean; results?: FixApplyResult[] }

interface FixBridge {
  PlanFixes(): Promise<FixPlanReport>;
  ApplyFixes(): Promise<FixPlanReport>;
}

/**
 * Repairs run in the process that owns the library, so they are offered only
 * where that is this process. A browser, or a -gui-only window rendering a
 * separate notriosd, gets the command instead of a button — and gets told why,
 * rather than finding the section missing.
 */
function fixBridge(): FixBridge | undefined {
  const bound = (window as Window & { go?: { main?: { NativeUIBridge?: Partial<FixBridge> } } })
    .go?.main?.NativeUIBridge;
  return bound?.PlanFixes && bound?.ApplyFixes ? (bound as FixBridge) : undefined;
}

/** Turns `missing_alt_text` into `missing alt text`. */
function readable(check: string): string {
  return check.replaceAll('_', ' ');
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB'];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(1)} ${units[unit]}`;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function LibraryHealth({ onClose }: { onClose: () => void }) {
  const [lint, setLint] = useState<LintReport | null>(null);
  const [garbage, setGarbage] = useState<GarbageCollectionReport | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [fixPlan, setFixPlan] = useState<FixPlan | null>(null);
  const [fixResults, setFixResults] = useState<FixApplyResult[] | null>(null);
  const [fixBusy, setFixBusy] = useState(false);
  const [fixError, setFixError] = useState('');
  const repairs = fixBridge();

  const refresh = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      // Both, or neither: a health screen showing one report and silently
      // omitting the other reads as a clean library.
      const [lintReport, garbageReport] = await Promise.all([getLintReport(), getGarbageReport()]);
      setLint(lintReport);
      setGarbage(garbageReport);
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [refresh, onClose]);

  const problems = lint?.checks.filter((check) => check.count > 0) ?? [];

  const runFixes = useCallback(async (apply: boolean) => {
    if (!repairs) return;
    setFixBusy(true);
    setFixError('');
    try {
      const report = apply ? await repairs.ApplyFixes() : await repairs.PlanFixes();
      setFixPlan(report.plan);
      setFixResults(report.results ?? null);
      // The lint report describes a library that has just changed.
      if (apply) void refresh();
    } catch (caught) {
      setFixError(errorMessage(caught));
    } finally {
      setFixBusy(false);
    }
  }, [repairs, refresh]);

  return (
    <div className="sync-center-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="sync-center sync-center-single" role="dialog" aria-modal="true"
        aria-labelledby="library-health-title" data-testid="library-health">
        <header className="sync-center-header">
          <div>
            <p className="eyebrow">This library</p>
            <h2 id="library-health-title">Library health</h2>
            <p className="muted">Nothing on this screen changes anything. Both reports are read-only.</p>
          </div>
          <button type="button" className="icon-button sync-close" aria-label="Close library health"
            onClick={onClose}>×</button>
        </header>

        <div className="sync-center-body">
          {loading && <p className="muted" data-testid="library-health-loading">Reading the library…</p>}
          {error && <p className="sync-inline-notice error" role="status"
            data-testid="library-health-error">{error}</p>}

          {lint && (
            <div className="sync-section" data-testid="library-health-lint">
              <h3>What has rotted</h3>
              {problems.length === 0 ? (
                <p className="success-line" data-testid="library-health-clean">
                  ✓ Nothing found across {lint.checks.length} checks.
                </p>
              ) : (
                <>
                  <p><strong>{lint.total_findings}</strong> findings across{' '}
                    <strong>{problems.length}</strong> of {lint.checks.length} checks.</p>
                  <ul className="health-checks">
                    {problems.map((check) => (
                      <li key={check.check} data-testid={`library-health-check-${check.check}`}>
                        <strong>{check.count}</strong> · {readable(check.check)}
                        {/* The count is complete; only the examples are capped.
                            Saying so stops a reader treating a truncated list
                            as the whole problem. */}
                        {check.truncated && <span className="muted"> · showing the first {check.findings.length}</span>}
                      </li>
                    ))}
                  </ul>
                </>
              )}
              {lint.warnings.length > 0 && (
                <ul className="report-warnings" data-testid="library-health-lint-warnings">
                  {lint.warnings.map((warning) => <li key={warning}>{warning}</li>)}
                </ul>
              )}
              <p className="boundary-note">
                Not everything here can be repaired mechanically; what can is below.
              </p>
            </div>
          )}

          <div className="sync-section" data-testid="library-health-repairs">
            <h3>What can be repaired mechanically</h3>
            <p className="muted">
              Some findings can be corrected without judgement. Each repair writes a revision against the
              note it was computed from, so a note edited in the meantime refuses rather than being
              repaired against text nobody looked at.
            </p>
            {repairs ? (
              <>
                <div className="row-actions">
                  <button type="button" data-testid="fix-plan" disabled={fixBusy}
                    onClick={() => void runFixes(false)}>Show me what would be repaired</button>
                  <button type="button" className="primary-button" data-testid="fix-apply"
                    disabled={fixBusy || fixPlan === null || fixPlan.total_edits === 0}
                    onClick={() => void runFixes(true)}>Repair them</button>
                </div>
                {fixPlan && (
                  <div className="tag-rename-report" data-testid="fix-plan-report">
                    {fixPlan.total_edits === 0 ? (
                      <p data-testid="fix-nothing">Nothing here can be repaired mechanically.</p>
                    ) : (
                      <p>
                        <strong>{fixPlan.total_edits}</strong> edits across{' '}
                        <strong>{fixPlan.total_documents}</strong>{' '}
                        {fixPlan.total_documents === 1 ? 'note' : 'notes'}
                        {fixPlan.truncated && <span className="muted"> · showing the first {fixPlan.documents.length}</span>}.
                      </p>
                    )}
                    {fixResults && (
                      <ul data-testid="fix-results">
                        {fixResults.map((result) => (
                          <li key={result.document_id}>
                            <code>{result.document_id}</code>{' '}
                            {result.error
                              ? <span className="tag-rename-merge">refused: {result.error}</span>
                              : <span className="muted">{result.applied} repaired
                                  {result.skipped > 0 ? `, ${result.skipped} skipped` : ''}</span>}
                          </li>
                        ))}
                      </ul>
                    )}
                    {fixPlan.warnings.length > 0 && (
                      <ul className="report-warnings" data-testid="fix-warnings">
                        {fixPlan.warnings.map((warning) => <li key={warning}>{warning}</li>)}
                      </ul>
                    )}
                  </div>
                )}
                {fixError && <p className="sync-inline-notice error" role="status"
                  data-testid="fix-error">{fixError}</p>}
              </>
            ) : (
              <p className="boundary-note" data-testid="fix-unavailable">
                Repairing needs the desktop application: it runs in the program that owns the library, and
                this window is showing one owned by another. From a terminal it is <code>notriosctl fix</code>,
                which reports without <code>--apply</code>.
              </p>
            )}
          </div>

          {garbage && (
            <div className="sync-section" data-testid="library-health-gc">
              <h3>What could be reclaimed</h3>
              <p>
                <strong>{garbage.eligible.length}</strong>{' '}
                {garbage.eligible.length === 1 ? 'attachment is' : 'attachments are'} unreferenced and past
                their retention window, out of {garbage.referenced_resource_count} still in use.
                {garbage.retained.length > 0 && (
                  <> {garbage.retained.length} more{' '}
                    {garbage.retained.length === 1 ? 'is' : 'are'} unreferenced but still retained.</>
                )}
              </p>
              {garbage.bytes_removed > 0 && (
                <p className="muted">{formatBytes(garbage.bytes_removed)} would be freed.</p>
              )}
              {garbage.warnings.length > 0 && (
                <ul className="report-warnings" data-testid="library-health-gc-warnings">
                  {garbage.warnings.map((warning) => <li key={warning}>{warning}</li>)}
                </ul>
              )}
              <p className="boundary-note">
                Deleting them is <code>notriosctl gc --apply</code>. The service has no apply for this on
                purpose: reclaiming space should be an explicit local act.
              </p>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
