import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  cancelSyncJob,
  createPasswordBackup,
  createSyncInvitation,
  discoverSyncPeers,
  getSyncConflict,
  getSyncUIStatus,
  initializeSync,
  inspectPasswordBackup,
  pairSyncPeer,
  requestSyncRecovery,
  resolveSyncConflict,
  retrySyncJob,
  saveSyncConfiguration,
  setSyncResourceIntent,
  setSyncSnapshotPermission,
  startSync,
  type BackupReview,
  type SyncUIConflictDetail,
  type SyncUIStatus,
} from '../api';
import { errorMessage } from '../preview-utils';

type SyncTab = 'overview' | 'setup' | 'peers' | 'resources' | 'conflicts' | 'backup' | 'repairs';

const tabs: Array<{ id: SyncTab; label: string }> = [
  { id: 'overview', label: 'Overview' },
  { id: 'setup', label: 'Setup' },
  { id: 'peers', label: 'Peers' },
  { id: 'resources', label: 'Attachments' },
  { id: 'conflicts', label: 'Conflicts' },
  { id: 'backup', label: 'Backup & recovery' },
  { id: 'repairs', label: 'Repairs' },
];

function friendlyKind(kind: string) {
  return kind.replace(/^sync_/, '').replaceAll('_', ' ');
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KiB`;
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MiB`;
  return `${(value / 1024 ** 3).toFixed(1)} GiB`;
}

function jobProgress(processed: number, total: number) {
  if (total <= 0) return undefined;
  return Math.max(0, Math.min(100, Math.round((processed / total) * 100)));
}

export interface SyncCenterProps {
  onClose: () => void;
}

export function SyncCenter({ onClose }: SyncCenterProps) {
  const [tab, setTab] = useState<SyncTab>('overview');
  const [status, setStatus] = useState<SyncUIStatus | null>(null);
  const [busy, setBusy] = useState('');
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const closeRef = useRef<HTMLButtonElement>(null);

  const refresh = useCallback(async () => {
    try {
      setStatus(await getSyncUIStatus());
    } catch (err) {
      setError(errorMessage(err));
    }
  }, []);

  useEffect(() => {
    void refresh();
    closeRef.current?.focus();
    const interval = window.setInterval(() => void refresh(), 5_000);
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', escape);
    return () => {
      window.clearInterval(interval);
      window.removeEventListener('keydown', escape);
    };
  }, [onClose, refresh]);

  const act = useCallback(async (name: string, operation: () => Promise<unknown>, success: string) => {
    setBusy(name);
    setError('');
    setNotice('');
    try {
      await operation();
      setNotice(success);
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy('');
    }
  }, [refresh]);

  const conflictCount = status?.conflicts.length ?? 0;
  const missingCount = status?.resources.filter((resource) => resource.availability === 'unavailable').length ?? 0;

  return (
    <div className="sync-center-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="sync-center" role="dialog" aria-modal="true" aria-labelledby="sync-center-title">
        <header className="sync-center-header">
          <div>
            <p className="eyebrow">Local synchronization</p>
            <h2 id="sync-center-title">Sync center</h2>
            {status ? (
              <p className="sync-profile-identity">
                <strong>{status.active_profile.name}</strong>
                <span>Library {status.active_profile.database_id.slice(0, 12)}…</span>
                <span>Replica {status.active_profile.replica_id.slice(0, 12)}…</span>
              </p>
            ) : <p className="muted">Reading this profile…</p>}
          </div>
          <button ref={closeRef} type="button" className="icon-button sync-close" aria-label="Close sync center" onClick={onClose}>×</button>
        </header>

        <nav className="sync-tabs" aria-label="Synchronization sections">
          {tabs.map((item) => (
            <button key={item.id} type="button" aria-current={tab === item.id ? 'page' : undefined}
              aria-label={item.id === 'conflicts' && conflictCount > 0
                ? `${item.label} ${conflictCount}`
                : item.id === 'resources' && missingCount > 0
                  ? `${item.label} ${missingCount}`
                  : item.label}
              className={tab === item.id ? 'active' : ''} onClick={() => setTab(item.id)}>
              {item.label}
              {item.id === 'conflicts' && conflictCount > 0 ? <span className="count-badge">{conflictCount}</span> : null}
              {item.id === 'resources' && missingCount > 0 ? <span className="count-badge">{missingCount}</span> : null}
            </button>
          ))}
        </nav>

        <div className="sync-center-body">
          {(error || notice) ? <div className={`sync-inline-notice ${error ? 'error' : ''}`} role="status">{error || notice}</div> : null}
          {!status ? <div className="sync-loading" role="status">Loading synchronization state…</div> : null}
          {status && tab === 'overview' ? <Overview status={status} busy={busy} act={act} onTab={setTab} /> : null}
          {status && tab === 'setup' ? <Setup status={status} busy={busy} act={act} /> : null}
          {status && tab === 'peers' ? <Peers status={status} busy={busy} act={act} setNotice={setNotice} setError={setError} /> : null}
          {status && tab === 'resources' ? <Resources status={status} busy={busy} act={act} /> : null}
          {status && tab === 'conflicts' ? <Conflicts status={status} busy={busy} refresh={refresh} setBusy={setBusy} setNotice={setNotice} setError={setError} /> : null}
          {status && tab === 'backup' ? <BackupRecovery status={status} busy={busy} act={act} refresh={refresh} setBusy={setBusy} setNotice={setNotice} setError={setError} /> : null}
          {status && tab === 'repairs' ? <Repairs status={status} /> : null}
        </div>
      </section>
    </div>
  );
}

interface SectionProps {
  status: SyncUIStatus;
  busy: string;
  act: (name: string, operation: () => Promise<unknown>, success: string) => Promise<void>;
}

function Overview({ status, busy, act, onTab }: SectionProps & { onTab: (tab: SyncTab) => void }) {
  const activeJob = status.jobs.find((job) => job.state === 'running' || job.state === 'queued');
  return (
    <div className="sync-section">
      <div className="sync-hero-row">
        <div className={`sync-state-orb ${status.status}`} aria-hidden="true" />
        <div>
          <h3>{status.status === 'ready' ? 'Up to date' : status.status === 'setup' ? 'Finish setup' : status.status === 'offline' ? 'Offline—will retry' : status.status === 'syncing' ? 'Synchronization in progress' : 'Needs attention'}</h3>
          <p>{status.configuration.target === 'none' ? 'No transport is selected.' : `Using ${status.configuration.target === 'rest' ? 'a REST peer' : 'a shared directory'}.`}</p>
        </div>
        <button type="button" className="primary-button" disabled={busy !== '' || status.configuration.target === 'none'}
          onClick={() => void act('sync-now', () => startSync('incremental'), 'Synchronization was queued.')}>
          Sync now
        </button>
      </div>

      <div className="sync-metric-grid">
        <button type="button" onClick={() => onTab('peers')}><strong>{status.peers.length}</strong><span>paired peers</span></button>
        <button type="button" onClick={() => onTab('resources')}><strong>{status.resources.filter((item) => item.availability === 'unavailable').length}</strong><span>missing attachments</span></button>
        <button type="button" onClick={() => onTab('conflicts')}><strong>{status.conflicts.length}</strong><span>conflicts</span></button>
        <button type="button" onClick={() => onTab('repairs')}><strong>{status.repairs.length}</strong><span>repair reports</span></button>
      </div>

      {activeJob ? (
        <article className="sync-card" aria-label="Current synchronization job">
          <div className="sync-card-heading"><h4>{friendlyKind(activeJob.kind)}</h4><span className={`state-chip ${activeJob.state}`}>{activeJob.state}</span></div>
          <p>{activeJob.phase ? `Phase: ${friendlyKind(activeJob.phase)}` : 'Waiting for a worker checkpoint…'}</p>
          {jobProgress(activeJob.processed, activeJob.total) !== undefined ? <progress max="100" value={jobProgress(activeJob.processed, activeJob.total)} /> : null}
          <button type="button" disabled={busy !== '' || activeJob.cancel_requested}
            onClick={() => void act(`cancel-${activeJob.id}`, () => cancelSyncJob(activeJob.id), 'Cancellation requested; the job will stop at a safe checkpoint.')}>
            {activeJob.cancel_requested ? 'Stopping safely…' : 'Cancel'}
          </button>
        </article>
      ) : null}

      <div className="sync-list" aria-label="Recent synchronization jobs">
        {status.jobs.slice(0, 6).map((job) => (
          <article key={job.id} className="sync-list-row">
            <div><strong>{friendlyKind(job.kind)}</strong><small>{job.phase ? friendlyKind(job.phase) : new Date(job.created_at).toLocaleString()}</small></div>
            <span className={`state-chip ${job.state}`}>{job.state}</span>
            {job.state === 'failed' || job.state === 'cancelled' ? (
              <button type="button" disabled={busy !== ''} onClick={() => void act(`retry-${job.id}`, () => retrySyncJob(job.id), 'The job was queued to retry from its verified checkpoint.')}>Retry</button>
            ) : null}
          </article>
        ))}
        {status.jobs.length === 0 ? <p className="sync-empty">No synchronization has been requested yet.</p> : null}
      </div>
    </div>
  );
}

function Setup({ status, busy, act }: SectionProps) {
  const [target, setTarget] = useState(status.configuration.target);
  const [directory, setDirectory] = useState(status.configuration.directory ?? '');
  const [restURL, setRestURL] = useState(status.configuration.rest_base_url ?? '');
  const [restInboundEnabled, setRESTInboundEnabled] = useState(status.configuration.rest_inbound_enabled);
  const nativeChooser = (window as Window & { go?: { main?: { NativeUIBridge?: { ChooseSyncDirectory?: () => Promise<string> } } } }).go?.main?.NativeUIBridge?.ChooseSyncDirectory;
  const activeProfile = status.profiles.find((profile) => profile.active);
  return (
    <div className="sync-section sync-form-section">
      <h3>Profile and transport</h3>
      <p>Each profile is an isolated local process. Switching opens the selected profile at its own local address.</p>
      <label>Active profile
        <select value={activeProfile?.name ?? status.active_profile.name} onChange={(event) => {
          const selected = status.profiles.find((profile) => profile.name === event.target.value);
          if (selected && !selected.active && selected.public_base_url) window.location.assign(selected.public_base_url);
        }}>
          {status.profiles.map((profile) => <option key={profile.name} value={profile.name}>{profile.name}{profile.active ? ' — active' : ''}</option>)}
        </select>
      </label>

      <div className="warning-card" role="note"><strong>{status.secret_store.name}</strong><p>{status.secret_store.warning}</p></div>
      {!status.journal_enabled || !status.secret_store.configured ? (
        <button type="button" className="primary-button" disabled={busy !== '' || !status.secret_store.available}
          onClick={() => void act('initialize', initializeSync, 'Synchronization was initialized. Restart this profile before pairing or syncing.')}>
          Initialize this profile
        </button>
      ) : <p className="success-line">✓ This profile has an enrolled journal and secret provider.</p>}

      <fieldset>
        <legend>Sync target</legend>
        <label className="radio-row"><input type="radio" name="sync-target" checked={target === 'none'} onChange={() => setTarget('none')} /> None</label>
        <label className="radio-row"><input type="radio" name="sync-target" checked={target === 'directory'} onChange={() => setTarget('directory')} /> Shared directory</label>
        <label className="radio-row"><input type="radio" name="sync-target" checked={target === 'rest'} onChange={() => setTarget('rest')} /> REST peer</label>
      </fieldset>
      {target === 'directory' ? <label>Shared directory on this device
        <span className="directory-picker"><input value={directory} onChange={(event) => setDirectory(event.target.value)} placeholder="/absolute/path/to/shared-folder" autoComplete="off" />
          {nativeChooser ? <button type="button" onClick={() => void nativeChooser().then((path) => { if (path) setDirectory(path); })}>Choose folder…</button> : null}
        </span>
        <small>Choose a disposable carrier folder. Your notes remain in the profile database.</small>
      </label> : null}
      {target === 'rest' ? <>
        <label>Peer address
          <input type="url" value={restURL} onChange={(event) => setRestURL(event.target.value)} placeholder="https://peer.example" autoComplete="off" />
          <small>HTTPS is required except between loopback profiles on this computer.</small>
        </label>
        <label className="checkbox-row"><input type="checkbox" checked={restInboundEnabled} onChange={(event) => setRESTInboundEnabled(event.target.checked)} /> Allow enrolled peers to connect to this profile</label>
        <small>This explicitly enables only the authenticated sync surface after restart; the general API remains local.</small>
      </> : null}
      <button type="button" className="primary-button" disabled={busy !== '' || target === 'directory' && !directory.trim() || target === 'rest' && !restURL.trim()}
        onClick={() => void act('save-configuration', () => saveSyncConfiguration({ target, directory, rest_base_url: restURL, rest_inbound_enabled: restInboundEnabled }), 'Configuration saved. Restart this profile to activate it.')}>
        Save transport
      </button>
      <p className="boundary-note">Changing a transport never enrolls a peer and never starts synchronization by itself.</p>
    </div>
  );
}

function Peers({ status, busy, act, setNotice, setError }: SectionProps & { setNotice: (value: string) => void; setError: (value: string) => void }) {
  const [baseURL, setBaseURL] = useState(status.configuration.rest_base_url ?? '');
  const [code, setCode] = useState('');
  const [label, setLabel] = useState('');
  const [invitation, setInvitation] = useState<{ code: string; expires_at: string } | null>(null);
  const [discovery, setDiscovery] = useState<string[]>([]);
  async function invite() {
    setError('');
    try {
      const next = await createSyncInvitation(label);
      setInvitation(next);
      setNotice('A single-use invitation is ready. Read the code to the other device over a separate channel.');
    } catch (err) { setError(errorMessage(err)); }
  }
  async function discover() {
    setError('');
    try {
      const result = await discoverSyncPeers();
      const candidates = (result.candidates ?? []).map((candidate) => String(candidate.replica_id ?? candidate.signer_key_id ?? 'Unknown candidate'));
      setDiscovery([...(result.peers ?? []), ...candidates]);
      setNotice(`Discovery scanned ${result.scanned_artifacts ?? 0} carrier artifacts without enrolling anyone.`);
    } catch (err) { setError(errorMessage(err)); }
  }
  return (
    <div className="sync-section">
      <h3>Paired peers</h3>
      <div className="sync-list">
        {status.peers.map((peer) => <article key={peer.replica_id} className="sync-list-row peer-row">
          <div><strong>{peer.replica_id.slice(0, 16)}…</strong><small>{peer.behind_operations > 0 ? `${peer.behind_operations} operations behind` : 'Caught up at last acknowledgement'}</small></div>
          <span className={`state-chip ${peer.status}`}>{peer.status}</span>
          <button type="button" disabled={busy !== '' || peer.status !== 'active' && peer.status !== 'behind'} aria-pressed={peer.snapshot_permitted}
            onClick={() => void act(`snapshot-permission-${peer.replica_id}`, () => setSyncSnapshotPermission(peer.replica_id, !peer.snapshot_permitted),
              peer.snapshot_permitted ? 'Catch-up snapshots are no longer allowed for this peer.' : 'This peer may now request a complete catch-up snapshot.')}>
            {peer.snapshot_permitted ? 'Disallow snapshot' : 'Allow catch-up snapshot'}
          </button>
        </article>)}
        {status.peers.length === 0 ? <p className="sync-empty">No peers are enrolled. Discovery never enrolls one automatically.</p> : null}
      </div>
      <div className="sync-two-column">
        <article className="sync-card">
          <h4>Invite a peer</h4>
          <label>Private label<input value={label} maxLength={128} onChange={(event) => setLabel(event.target.value)} placeholder="Laptop" /></label>
          <button type="button" disabled={busy !== '' || !status.configuration.rest_inbound_enabled} onClick={() => void invite()}>Create 15-minute code</button>
          {!status.configuration.rest_inbound_enabled ? <small>Inbound REST sync must be enabled in this profile before it can invite.</small> : null}
          {invitation ? <div className="pairing-code" aria-live="polite"><code>{invitation.code}</code><button type="button" onClick={() => {
            void navigator.clipboard?.writeText(invitation.code);
            setNotice('Pairing code copied. Send it separately from the peer address.');
          }}>Copy code</button><small>Expires {new Date(invitation.expires_at).toLocaleTimeString()}</small></div> : null}
        </article>
        <article className="sync-card">
          <h4>Join an inviting peer</h4>
          <label>Peer address<input type="url" value={baseURL} onChange={(event) => setBaseURL(event.target.value)} placeholder="http://127.0.0.1:8081" /></label>
          <label>Single-use code<input value={code} onChange={(event) => setCode(event.target.value.toUpperCase())} autoComplete="one-time-code" spellCheck={false} /></label>
          <button type="button" className="primary-button" disabled={busy !== '' || !baseURL.trim() || !code.trim()}
            onClick={() => void act('pair', async () => { await pairSyncPeer(baseURL, code); setCode(''); }, 'The peer is paired. Queue synchronization when you are ready.')}>Pair explicitly</button>
        </article>
      </div>
      <button type="button" disabled={busy !== '' || status.configuration.target === 'none'} onClick={() => void discover()}>Discover on configured carrier</button>
      {discovery.length > 0 ? <ul className="discovery-list">{discovery.map((peer) => <li key={peer}>{peer}</li>)}</ul> : null}
    </div>
  );
}

function Resources({ status, busy, act }: SectionProps) {
  return <div className="sync-section"><h3>Lazy attachments</h3><p>Missing bytes are fetched only when you ask or pin them. Metadata remains usable while offline.</p>
    <div className="sync-list">{status.resources.map((resource) => <article key={resource.id} className="sync-list-row resource-row">
      <div><strong>{resource.filename || resource.id}</strong><small>{resource.mime_type} · {formatBytes(resource.size_bytes)}</small></div>
      <span className={`state-chip ${resource.availability === 'local' ? 'succeeded' : resource.requested ? 'queued' : 'offline'}`}>{resource.availability === 'local' ? 'available' : resource.requested ? 'pending' : 'not on this device'}</span>
      <div className="row-actions">
        {resource.availability !== 'local' ? <button type="button" disabled={busy !== ''} onClick={() => void act(`fetch-${resource.id}`, () => setSyncResourceIntent(resource.id, resource.pinned, true), `Requested ${resource.filename || 'the attachment'}; a bounded fetch job was queued when the target was available.`)}>Download</button> : null}
        <button type="button" disabled={busy !== ''} aria-pressed={resource.pinned} onClick={() => void act(`pin-${resource.id}`, () => setSyncResourceIntent(resource.id, !resource.pinned, resource.requested || !resource.pinned), resource.pinned ? 'The attachment is no longer pinned.' : 'The attachment is pinned to this device.')}>{resource.pinned ? 'Unpin' : 'Pin'}</button>
      </div>
    </article>)}{status.resources.length === 0 ? <p className="sync-empty">No missing or pinned attachments.</p> : null}</div>
  </div>;
}

function Conflicts({ status, busy, refresh, setBusy, setNotice, setError }: { status: SyncUIStatus; busy: string; refresh: () => Promise<void>; setBusy: (value: string) => void; setNotice: (value: string) => void; setError: (value: string) => void }) {
  const [detail, setDetail] = useState<SyncUIConflictDetail | null>(null);
  const [resolution, setResolution] = useState('');
  const [title, setTitle] = useState('');
  async function open(conflictID: string) {
    setBusy(`conflict-${conflictID}`); setError('');
    try {
      const next = await getSyncConflict(conflictID);
      setDetail(next); setResolution(next.local.body); setTitle(next.title);
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(''); }
  }
  async function resolve() {
    if (!detail) return;
    setBusy('resolve-conflict'); setError('');
    try {
      await resolveSyncConflict(detail.id, title, resolution);
      setDetail(null); setNotice('Conflict resolved as a two-parent revision. It will travel on the next sync.');
      await refresh();
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(''); }
  }
  if (detail) return <div className="sync-section conflict-detail"><button type="button" className="back-button" onClick={() => setDetail(null)}>← All conflicts</button><h3>{detail.title}</h3><p>Compare the common base, this replica’s displayed head, and the other head. Nothing is chosen automatically.</p>
    <div className="conflict-columns">
      <label>Common base<textarea readOnly value={detail.base.body} /></label>
      <label>Current on this profile<textarea readOnly value={detail.local.body} /></label>
      <label>Other revision<textarea readOnly value={detail.remote.body} /></label>
    </div>
    <label>Resolved title<input value={title} onChange={(event) => setTitle(event.target.value)} /></label>
    <label>Your resolved note<textarea className="resolution-editor" value={resolution} onChange={(event) => setResolution(event.target.value)} /></label>
    <div className="row-actions"><button type="button" onClick={() => setResolution(detail.local.body)}>Use current</button><button type="button" onClick={() => setResolution(detail.remote.body)}>Use other</button><button type="button" className="primary-button" disabled={busy !== ''} onClick={() => void resolve()}>Save resolution</button></div>
  </div>;
  return <div className="sync-section"><h3>Visible conflicts</h3><p>Notrios will not guess when concurrent edits overlap.</p><div className="sync-list">{status.conflicts.map((conflict) => <button type="button" className="sync-list-row conflict-row" key={conflict.id} disabled={busy !== ''} onClick={() => void open(conflict.id)}><div><strong>{conflict.document_id}</strong><small>{friendlyKind(conflict.kind)} · {new Date(conflict.created_at).toLocaleString()}</small></div><span>Compare →</span></button>)}{status.conflicts.length === 0 ? <p className="sync-empty">No unresolved body conflicts.</p> : null}</div></div>;
}

function BackupRecovery({ status, busy, act, refresh, setBusy, setNotice, setError }: SectionProps & { refresh: () => Promise<void>; setBusy: (value: string) => void; setNotice: (value: string) => void; setError: (value: string) => void }) {
  const [createPassword, setCreatePassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [openPassword, setOpenPassword] = useState('');
  const [showOpen, setShowOpen] = useState(false);
  const [backupFile, setBackupFile] = useState<File | null>(null);
  const [review, setReview] = useState<BackupReview | null>(null);
  const createValid = createPassword.length >= 8 && createPassword === confirmPassword;
  async function createBackup() {
    setBusy('create-backup'); setError(''); setNotice('');
    try {
      const blob = await createPasswordBackup(createPassword);
      const href = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = href; link.download = 'notrios-backup.npb'; link.click();
      window.setTimeout(() => URL.revokeObjectURL(href), 0);
      setNotice('Verified encrypted backup created. The password was not remembered.');
    } catch (err) { setError(errorMessage(err)); } finally {
      setCreatePassword(''); setConfirmPassword(''); setShowCreate(false); setBusy('');
    }
  }
  async function inspect() {
    if (!backupFile) return;
    setBusy('inspect-backup'); setError(''); setNotice(''); setReview(null);
    try {
      setReview(await inspectPasswordBackup(backupFile, openPassword));
      setNotice('Backup password accepted and the physical snapshot verified. Nothing was restored.');
    } catch (err) { setError(errorMessage(err)); } finally {
      setOpenPassword(''); setShowOpen(false); setBusy('');
    }
  }
  return <div className="sync-section"><h3>Backup and recovery</h3><div className="sync-two-column">
    <article className="sync-card"><h4>Create a password-protected backup</h4><p>The password wraps a fresh payload key with Argon2id. It is never saved.</p>
      <label>Password<span className="password-row"><input type={showCreate ? 'text' : 'password'} value={createPassword} minLength={8} maxLength={4096} autoComplete="new-password" onChange={(event) => setCreatePassword(event.target.value)} /><button type="button" aria-pressed={showCreate} onClick={() => setShowCreate((value) => !value)}>{showCreate ? 'Hide' : 'Show'}</button></span></label>
      <label>Confirm password<input type={showCreate ? 'text' : 'password'} value={confirmPassword} autoComplete="new-password" onChange={(event) => setConfirmPassword(event.target.value)} /></label>
      <button type="button" className="primary-button" disabled={busy !== '' || !createValid} onClick={() => void createBackup()}>Create and download</button>
      <small>Notrios cannot recover a forgotten backup password.</small>
    </article>
    <article className="sync-card"><h4>Review a backup for restore</h4><p>A wrong password changes nothing; retry or cancel safely.</p>
      <label>Backup file<input type="file" accept=".npb,application/vnd.notrios.password-backup" onChange={(event) => { setBackupFile(event.target.files?.[0] ?? null); setReview(null); }} /></label>
      <label>Password<span className="password-row"><input type={showOpen ? 'text' : 'password'} value={openPassword} autoComplete="off" onChange={(event) => setOpenPassword(event.target.value)} /><button type="button" aria-pressed={showOpen} onClick={() => setShowOpen((value) => !value)}>{showOpen ? 'Hide' : 'Show'}</button></span></label>
      <div className="row-actions"><button type="button" className="primary-button" disabled={busy !== '' || !backupFile || !openPassword} onClick={() => void inspect()}>Verify for review</button><button type="button" disabled={busy !== ''} onClick={() => { setBackupFile(null); setOpenPassword(''); setReview(null); }}>Cancel</button></div>
    </article>
  </div>
  {review ? <article className="destructive-review" role="region" aria-label="Destructive restore review"><h4>Verified—destructive review required</h4><dl><div><dt>Library</dt><dd>{review.database_id}</dd></div><div><dt>Snapshot</dt><dd>{review.snapshot_id}</dd></div><div><dt>Schema</dt><dd>{review.schema_version}</dd></div><div><dt>External objects</dt><dd>{review.objects}</dd></div></dl><p>Replace discards this profile’s canonical state after making and verifying an emergency snapshot. Adopt also mints a fresh replica identity. This review did not apply either action.</p><button type="button" disabled title="Close the active profile and use the explicit native restore handoff">Apply requires profile shutdown</button></article> : null}
  <article className="sync-card recovery-card"><h4>Catch up or reset from an enrolled REST peer</h4><p>Catch-up downloads and verifies a physical snapshot into private staging. Reset requests the same artifact but remains blocked on a separate destructive review.</p><div className="row-actions"><button type="button" disabled={busy !== '' || status.configuration.target === 'none'} onClick={() => void act('catchup', () => requestSyncRecovery('catchup'), 'Catch-up was queued with resumable transfer and verification.')}>Request catch-up</button><button type="button" className="danger-button" disabled={busy !== '' || status.configuration.target === 'none'} onClick={() => {
    if (window.confirm('Request a reset snapshot? Nothing will be replaced until a separate destructive review.')) void act('reset', () => requestSyncRecovery('reset'), 'Reset preparation was queued. No canonical data was changed.').then(refresh);
  }}>Request reset review</button></div></article>
  </div>;
}

function Repairs({ status }: { status: SyncUIStatus }) {
  return <div className="sync-section"><h3>Notebook repair reports</h3><p>Deterministic repairs preserve convergence but remain visible so you can review what changed.</p><div className="sync-list">{status.repairs.map((repair) => <article className="sync-list-row repair-row" key={repair.id}><div><strong>{friendlyKind(repair.kind)}</strong><small>{repair.subject_id}</small></div><pre>{JSON.stringify(repair.details ?? {}, null, 2)}</pre></article>)}{status.repairs.length === 0 ? <p className="sync-empty">No synchronization repairs have been recorded.</p> : null}</div></div>;
}
