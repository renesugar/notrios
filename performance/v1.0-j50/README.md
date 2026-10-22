# v1.0 J50: clear vitest's mocker advisories

J9's archive build stopped at `npm audit` in `web/` on 2026-10-21, on two
moderate advisories against `@vitest/mocker` `<=4.1.11`, which `vitest`
depends on at the same version. `docs-site/` had none. The plan is in
`PLAN.md` under J50.

| advisory | severity | patched |
|---|---|---|
| GHSA-qxg6-89mg-rqv8 | moderate | 4.1.12 |
| GHSA-xp26-3454-mjgp | moderate | 4.1.12 |

## J50-A: vitest 4.1.12, in the lockfile only

**What changed.** The lockfile moves eight entries from 4.1.11 to 4.1.12:
`vitest`, and `@vitest/expect`, `mocker`, `pretty-format`, `runner`,
`snapshot`, `spy` and `utils`. Each one changes its version, tarball and
integrity, and its exact pins on its siblings. All are MIT, as before. Nothing
else moved, and the package count stays 357. `package.json` is unchanged:
`^4.1.11` already admits 4.1.12.

**How the lockfile was moved, because npm did not do it the obvious way.** npm
12.0.2's `npm update vitest` reported "up to date", with and without
`--prefer-online`, although `npm outdated` listed 4.1.12 as wanted.
`npm install vitest@4.1.12` did move it. It also wrote `^4.1.12` into
`package.json`, and re-resolved two packages to newer versions in range:
`chai` 6.2.2 → 6.3.0 and `tinyrainbow` 3.1.1 → 3.2.0. 4.1.12 raises neither
floor; `@vitest/expect` asks for `chai ^6.2.2` and `tinyrainbow ^3.1.0` in both
versions. So three things were put back as they were:
- `package.json`;
- the lockfile's copy of the root's range;
- those two entries.

After that, a clean `npm ci` resolves `chai@6.2.2` and `tinyrainbow@3.1.1`.

**After a clean `npm ci`:**
- `npm audit`: found 0 vulnerabilities.
- `npm test`: 39 files and 362 tests passed. No test was changed.
- `npm run build`, which runs `tsc --noEmit` first: passed.
- G20's licence gate passed on the new lockfile hash: 39 Go modules and 365
  npm packages.

**The bundle is the same bytes.** `dist/` was built under 4.1.11 before the
change and under 4.1.12 after it, each from a clean `npm ci`. Every file was
hashed with SHA-256 and the two lists compared:

| | files | bytes |
|---|---:|---:|
| vitest 4.1.11 | 285 | 7,837,453 |
| vitest 4.1.12 | 285 | 7,837,453 |

All 285 files are identical, which is what was expected: vitest runs the unit
tests and is not in the build.
