# v0.7 upgrade and rollback

Notrios 0.7.0 uses canonical SQLite schema **v27**. A new database is created
at v27. Opening an older supported database runs the ordered, restart-safe
migrations once: v19 journal and replica identity, v20 admission compatibility,
v21 convergent metadata, v22 revision/conflict objects, v23 lazy resource
materialization, v24 snapshot catch-up, v25 REST peer security, v26 durable
sync jobs, and v27 retirement/retention floors. The release gate runs the fresh
bootstrap and the existing v18-to-current plus every later migration test;
backfills are idempotent or explicitly resumable.

Before the first 0.7.0 open, stop every Notrios process that can own the
profile and make a verified, application-consistent snapshot of the database
and asset directory. Preserve the 0.6.0 binary/source release with that
snapshot. Do not copy a live WAL database and do not let two versions open the
same canonical files concurrently.

There is no in-place v27-to-v18 downgrade migration. To roll back:

1. stop all Notrios processes for the profile;
2. preserve the failed v27 profile separately for diagnosis;
3. verify the pre-upgrade snapshot digest and restore the database plus assets
   as one set into an empty profile directory;
4. start the preserved 0.6.0 release against only that restored profile; and
5. run `notriosctl doctor` before allowing writes or synchronization.

Changes made only after the v27 upgrade are not present in the pre-upgrade
snapshot. Exporting or translating those changes is a separate recovery task;
never manufacture a lower `PRAGMA user_version` or reuse v27 sync credentials
with a restored older replica. If the verified snapshot is unavailable, keep
the v27 profile read-only and recover through the typed snapshot/catch-up path
with an enrolled permitted peer rather than attempting an SQL downgrade.
