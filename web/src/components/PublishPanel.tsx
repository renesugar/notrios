// Publishing: choose a profile, read what it would let out, then agree to it.
//
// Publishing is not export. An export preserves enough to restore private
// state; a publication is a sanitized projection, and it is the one output that
// rewrites note content — a link to a note you are withholding cannot stay a
// link. So the review is not a courtesy step before the real one. It is the
// feature, and the run is bound to it: the digest shown here is checked against
// a plan recomputed at the moment of publishing, and a library edited in
// between is refused rather than published.
import { useCallback, useEffect, useState } from 'react';

interface PublishProfile { name: string; description: string; target: string }

interface PublicationPlan {
  profile: string;
  target: string;
  manifest_sha256: string;
  counts: Record<string, number>;
  exclusions: Array<{ kind: string; id: string; reason: string }>;
  warnings: string[];
  truncated: boolean;
  command: string;
}

interface PublicationResult {
  profile: string; directory: string; documents: number; objects: number; manifest_sha256: string;
}

export interface PublishBridge {
  ChooseDirectory(purpose: string): Promise<string>;
  PublishProfiles?: () => Promise<PublishProfile[] | null>;
  PlanPublication?: (name: string) => Promise<PublicationPlan>;
  Publish?: (name: string, reviewedDigest: string, path: string) => Promise<PublicationResult>;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** The counts a reader needs first, in the order they need them. */
const HEADLINE: Array<{ key: string; label: string; concerning?: boolean }> = [
  { key: 'selected_documents', label: 'notes published' },
  { key: 'excluded_documents', label: 'notes withheld' },
  { key: 'private_links', label: 'links to withheld notes, rewritten', concerning: true },
  { key: 'broken_links', label: 'links that resolve to nothing', concerning: true },
  { key: 'reachable_resources', label: 'attachments published' },
  { key: 'oversized_resources', label: 'attachments too large to publish', concerning: true },
];

export function PublishPanel({ bridge }: { bridge: PublishBridge | undefined }) {
  const [profiles, setProfiles] = useState<PublishProfile[] | null>(null);
  const [chosen, setChosen] = useState('');
  const [plan, setPlan] = useState<PublicationPlan | null>(null);
  const [directory, setDirectory] = useState('');
  const [result, setResult] = useState<PublicationResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!bridge?.PublishProfiles) return;
    void bridge.PublishProfiles()
      .then((list) => {
        // A Go nil slice arrives as null, not as an empty array. Reading
        // .length off it blanks the whole dialog, which is how the About
        // dialog broke.
        const found = list ?? [];
        setProfiles(found);
        if (found.length === 1) setChosen(found[0].name);
      })
      .catch(() => setProfiles([]));
  }, [bridge]);

  // Changing the profile invalidates the review. A digest belongs to one
  // profile, and offering to publish a review of a different one is exactly
  // what the reviewed-plan check exists to prevent.
  const choose = useCallback((name: string) => {
    setChosen(name);
    setPlan(null);
    setResult(null);
    setError('');
  }, []);

  const review = useCallback(async () => {
    if (!bridge?.PlanPublication || chosen === '') return;
    setBusy(true);
    setError('');
    try {
      setPlan(await bridge.PlanPublication(chosen));
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setBusy(false);
    }
  }, [bridge, chosen]);

  const publish = useCallback(async () => {
    if (!bridge?.Publish || plan === null) return;
    setBusy(true);
    setError('');
    try {
      setResult(await bridge.Publish(plan.profile, plan.manifest_sha256, directory.trim()));
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setBusy(false);
    }
  }, [bridge, plan, directory]);

  if (!bridge?.PublishProfiles) {
    return (
      <div className="sync-section" data-testid="transfer-publish">
        <h3>Publish a subset of your notes</h3>
        <p className="boundary-note" data-testid="publish-unavailable">
          Publishing writes to a folder on the machine that owns the library, so it needs the desktop
          application. From a terminal it is <code>notriosctl publish plan</code> and then{' '}
          <code>notriosctl publish run</code>.
        </p>
      </div>
    );
  }

  return (
    <div className="sync-section" data-testid="transfer-publish">
      <h3>Publish a subset of your notes</h3>
      <p className="muted">
        A publication is not a backup. It carries current versions only, no trashed notes, no history
        and no provenance, and it rewrites links that point at notes you are withholding.
      </p>

      {profiles !== null && profiles.length === 0 ? (
        <p className="boundary-note" data-testid="publish-no-profiles">
          No publication profiles are saved. A profile says what to include and what to strip; make one
          with <code>notriosctl publish profile save</code>. It is deliberately not a form here: a dozen
          choices quietly defaulted is how something private gets published.
        </p>
      ) : (
        <>
          <label>Publication
            <select value={chosen} data-testid="publish-profile"
              onChange={(event) => choose(event.target.value)}>
              <option value="">Choose a profile…</option>
              {(profiles ?? []).map((profile) => (
                <option key={profile.name} value={profile.name}>
                  {profile.name}{profile.description ? ` — ${profile.description}` : ''}
                </option>
              ))}
            </select>
          </label>

          <div className="row-actions">
            <button type="button" data-testid="publish-review" disabled={busy || chosen === ''}
              onClick={() => void review()}>Review what this would publish</button>
          </div>

          {plan && (
            <div className="tag-rename-report" data-testid="publish-plan">
              <ul className="health-checks">
                {HEADLINE.filter(({ key }) => (plan.counts[key] ?? 0) > 0 || key === 'selected_documents')
                  .map(({ key, label, concerning }) => (
                    <li key={key} data-testid={`publish-count-${key}`}>
                      <strong>{plan.counts[key] ?? 0}</strong> ·{' '}
                      <span className={concerning ? 'tag-rename-merge' : undefined}>{label}</span>
                    </li>
                  ))}
              </ul>
              {plan.exclusions.length > 0 && (
                <p className="muted" data-testid="publish-exclusions">
                  {plan.exclusions.length} {plan.exclusions.length === 1 ? 'exclusion' : 'exclusions'} recorded,
                  the first being “{plan.exclusions[0].reason}”.
                </p>
              )}
              {plan.warnings.length > 0 && (
                <ul className="report-warnings" data-testid="publish-warnings">
                  {plan.warnings.map((warning) => <li key={warning}>{warning}</li>)}
                </ul>
              )}
              {/* Shown, not hidden: the digest is what binds this review to the
                  publication, and it is what a person needs if they would
                  rather run it from a terminal. */}
              <p className="muted publish-digest" data-testid="publish-digest">
                Reviewed plan <code>{plan.manifest_sha256.slice(0, 16)}…</code>. Publishing checks this
                against the library as it is then; if it has changed, nothing is written.
              </p>
              <p className="muted publish-command"><code>{plan.command}</code></p>
            </div>
          )}

          <label>Folder to publish into
            <span className="directory-picker">
              <input value={directory} data-testid="publish-directory"
                onChange={(event) => setDirectory(event.target.value)}
                placeholder="/absolute/path/to/folder" autoComplete="off" />
              <button type="button" data-testid="publish-choose" disabled={busy}
                onClick={() => void bridge.ChooseDirectory('publish').then((path) => {
                  if (path) setDirectory(path);
                })}>Choose folder…</button>
            </span>
          </label>

          <div className="row-actions">
            <button type="button" className="primary-button" data-testid="publish-run"
              disabled={busy || plan === null || directory.trim() === ''}
              onClick={() => void publish()}>Publish this review</button>
          </div>

          {result && (
            <p className="success-line" data-testid="publish-result">
              Published {result.documents} {result.documents === 1 ? 'note' : 'notes'} into {result.directory}.
            </p>
          )}
          {error && <p className="sync-inline-notice error" role="status"
            data-testid="publish-error">{error}</p>}
        </>
      )}
    </div>
  );
}
