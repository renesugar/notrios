# SQLite image state review

This is the G14c table-by-table decision for schema v25. Deletion happens only
in the private Online Backup image, with `PRAGMA secure_delete=ON`, followed by
`VACUUM` so historical source freelist pages cannot retain older discarded
local bytes. The source database is unchanged.

## Canonical and database-wide state retained

- Core identity/content: `database_identity`, `collections`, `notebooks`,
  `search_notebooks`, `tags`, `documents`, `document_revisions`, `note_tags`,
  `blobs`, `resources`, `document_resource_refs`, `resource_hashes`,
  `document_sources`, and `source_bundle_items`.
- Database-wide media policy: `media_domain_rules`, `media_hash_rules`, and
  `media_policy_decisions`.
- Admitted history and convergence: `sync_replicas`,
  `sync_snapshot_boundaries`, `sync_local_journal`, `sync_operations`,
  `sync_operation_dependencies`, `sync_state_vectors`, `sync_audit_events`,
  `sync_peer_compatibility`, `sync_peer_keys`, `sync_metadata_baseline_floors`,
  `sync_metadata_baseline_memberships`, `sync_metadata_baseline_records`,
  `sync_field_registers`, `sync_membership_registers`,
  `sync_lifecycle_registers`, `sync_death_certificates`, `sync_hlc_clock`,
  `sync_document_conflicts`, `sync_revision_deltas`,
  `sync_revision_pending_bodies`, `sync_repair_events`, and
  `sync_catchup_floors`.
- `sync_blob_manifests` rows with `complete=1` are retained. They are immutable
  object-transfer metadata needed to advertise a restored local object again.

The copied local journal and replica identity are evidence of the source
boundary, not permission to allocate as that replica. G14d must retire that
allocator and mint a new replica before opening an installed image for writes.
Signing private keys and group-encryption keys remain in owner-only secret
files and never enter SQLite.

## Same-schema derived state retained but rebuildable

`document_blocks`, `document_links`, the `documents_fts` virtual table and its
SQLite shadow tables, and `index_outbox` are retained for exact-schema recovery.
They are non-authoritative. G14d must select a bounded rebuild if admission or
a future migration invalidates them. Recoll's external index is never packed.

## Securely cleared local/transient state

- Local job/batch/restore resumptions: `jobs`, `batch_operations`, and
  `restore_state`.
- Source-machine import resumptions and path material:
  `import_checkpoints` and `import_item_states`.
- Pairing/catch-up process state: `sync_pairing_invitations`,
  `sync_catchup_permissions`, and `sync_catchup_sessions`.
- Transaction and admission seams: `sync_revision_transfer`,
  `sync_apply_guard`, `sync_journal_capture`, `sync_pending_admissions`, and
  `sync_state_gaps`.
- Source-replica observations: `sync_peer_acknowledgements` and
  `sync_blob_sources`.
- Staged/materialization state: `sync_blob_chunks`,
  `sync_blob_materialization`, and incomplete (`complete=0`)
  `sync_blob_manifests`.

Only database-declared `availability='local'` blob files and declared source
bundle files enter deterministic packs. `staging/`, `incoming-*`, orphan
files, arbitrary asset-root entries, and external indexes are excluded.
