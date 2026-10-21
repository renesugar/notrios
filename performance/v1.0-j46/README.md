# v1.0 J46: clear vite's launch-editor advisories

J39's archive build stopped at `npm audit` in `web/` on 2026-10-21, on three
advisories published the day before against `launch-editor`, which vite bundles
to serve its dev server's open-in-editor endpoint. `docs-site/` had none. The
plan is in `PLAN.md` under J46.

| advisory | severity | patched for the 8.1 line |
|---|---|---|
| GHSA-587h-7wmj-h5pm | high | vite 8.1.6 |
| GHSA-r825-w8mm-5pxw | medium | vite 8.1.6 |
| GHSA-698v-4r3x-3vcv | medium | vite 8.1.6 |

## J46-A: vite 8.1.6, in the lockfile only

`npm install vite@8.1.6` wrote `"^8.1.6"` into `package.json`, so the file was
put back as it was, and the lockfile's copy of the root's ranges was set back to
`"latest"`. `npm ci` accepts the result, and `package.json` is unchanged.

The lockfile changes in one entry: vite's version 8.1.4 to 8.1.6, its tarball
and integrity, and its own ranges for `postcss` (`^8.5.16` to `^8.5.17`) and
`rolldown` (`~1.1.4` to `~1.1.5`). Nothing else moved. The lockfile already held
rolldown 1.1.5 and postcss 8.5.26, which satisfy the raised floors, so no other
package was installed, removed or changed. The package count stays 357.

After a clean `npm ci`:

- `npm audit`: found 0 vulnerabilities.
- `npm test`: 34 files and 305 tests passed.
- `npm run build`, which runs `tsc --noEmit` first: passed.
- G20's licence gate: passed, 39 Go modules and 365 npm packages, once its
  inventory carried the lockfile's new hash. vite is MIT, as before.

### The bundle is the same bytes

`dist/` was built from the same source under 8.1.4, before the change, and under
8.1.6, after it, each from a clean `npm ci`. Every file was hashed with SHA-256
and the two lists compared:

| | files | bytes |
|---|---:|---:|
| vite 8.1.4 | 285 | 7,831,037 |
| vite 8.1.6 | 285 | 7,831,037 |

All 285 files are identical. The UI Notrios ships is byte for byte what it was,
which is what the advisories' scope predicts: the fix is in the dev server, and
the bundler, rolldown, did not change version.

### Recorded, not decided (J8)

`package.json` asks for `"latest"` for vite and several other packages. That is
why `npm audit fix` named vite 8.3.5, published on 2026-10-21, the same morning,
as the fix, and called it outside the stated range: a tag has no range to stay
inside. The lockfile is what fixes a version, and `npm ci` is what the gate
runs, so nothing floats today. But any `npm install` that refreshes the lockfile
moves every `"latest"` dependency to whatever was published last, however new.
Whether the web workspace should state ranges for its build tools is a separate
question, and it is not decided here.

## J46-B: the gate passes

`scripts/package_release.sh` ran through from `cd14448`, the J46-A commit, in a
fresh worktree on HD2. `npm audit` was clean in `web/` and `docs-site/`, every
validation after it passed, and it wrote `notrios-v1.0-j39-cd14448.zip`:
26,659,678 bytes, 2,991 entries, accepted by `check_release_zip.py`. J39 was
closed from that commit, in `aaa101e`.

## Archive

`notrios-v1.0-j46-de1affb.zip`, built from commit `de1affb`, the commit that
completed this item's record, with the ledger not yet flipped. 26,660,213 bytes,
2,991 entries, accepted by `check_release_zip.py`. No usage override was needed:
the seven-day bucket had 93% remaining and the five-hour bucket had 94%.
