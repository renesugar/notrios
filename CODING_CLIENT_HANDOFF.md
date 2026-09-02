# Coding Client Handoff

This handoff applies to any coding agent or client continuing this project (Codex, Claude, aider, swival.dev, etc. — formerly `CODEX_HANDOFF.md`). The repository is designed so an agent can continue from repository files alone.

## History rewrite — 2026-09-01

`notrioslib`, an 11 MB compiled ELF executable, was committed by accident in the
v0.8 H1 ABI slice: `go build ./cmd/notrioslib` with no `-o` writes the binary
into the working directory, and a `git add -A` swept it up. It was referenced by
no Makefile target, script or document. It has been **removed from git history**
at the user's direction.

Every commit from the H1 ABI slice onward therefore has a new SHA. The rewrite
was local only — `origin/develop` was 167 commits behind and never contained it,
so no force-push was needed and nothing on GitHub changed. Verified afterwards:
197 commits before and after with identical subjects in order, and at the commit
that introduced the binary exactly one path differs, every other blob hash being
unchanged. `.git` is now 7.4 MB.

**The five contaminated evidence archives were deliberately left as they are.**
`notrios-v0.8-{h1,h2a,h2b,h2,h3}-*.zip` each contain the 11 MB binary and are
named after commits that no longer exist. They remain accurate records of the
tree as it stood when each slice completed. Use this table to relate them to the
current history:

| Archive / old SHA | Current SHA |
| --- | --- |
| `09dfbfe` | `e28abfd` |
| `0bee555` | `fa3803c` |
| `1a19a85` | `be6cb8f` |
| `27de1d1` | `e69419c` |
| `2b9f00b` | `af37c4c` |
| `2c939d6` | `0196596` |
| `37cadac` | `97652f1` |
| `3a851a3` | `fdd9ddc` |
| `3ace3af` | `e8e69c5` |
| `4811d0b` | `c457445` |
| `49f0fe7` | `55eed4d` |
| `5808176` | `771326d` |
| `5b2b215` | `85ce573` |
| `5e7cab9` | `e014bf8` |
| `6448c9b` | `6e4831b` |
| `6ab6aa6` | `8a28b5d` |
| `6d57d8e` | `623814a` |
| `6e0f617` | `2216483` |
| `9043a4d` | `e4f4432` |
| `c457fb8` | `f4b8c27` |
| `c7cca80` | `102df1c` |
| `cce64d5` | `5ebcb57` |
| `ce378d8` | `d61c524` |
| `e755321` | `d935c0f` |
| `e989ea6` | `14ac437` |

**The pre-rewrite history is preserved** as
`/home/renes/evidence/notrios/notrios-v0.8-develop-before-notrioslib-rewrite.bundle`
(14,301,956 bytes, SHA-256 `4e6cd6591baf4bf2818f9b5b690b9d68f259ae15be74e8ce15f3275cce285417`).
It is a complete, verified git bundle: cloning it restores all 197 commits at
their original SHAs, with the 11 MB binary present at the H1 ABI commit
`6d57d8e`. That commit is `623814a` in the current history and no longer
carries it. The bundle is the only remaining copy of the original SHAs, so the
mapping table above is how the archives stay addressable without it.

**The gap that let it ship is closed.** `scripts/check_release_zip.py` used to
forbid entries named `notrios`, `notriosd` and `notriosctl` — a list somebody
has to remember to extend. It now rejects any entry beginning with the ELF magic
bytes, which needs no maintenance. Run against the already-shipped H3 archive it
reports the binary immediately. `.gitignore` and `package_release.sh` also name
`/notrioslib` explicitly.

## v0.8 package snapshots — 2026-09-01

Every v0.8 slice now has a handoff ZIP in `dist/` and a byte-identical copy in
`/home/renes/evidence/notrios`:

| Slice | Commit | Bytes | Entries | SHA-256 (first 16) |
| --- | --- | --- | --- | --- |
| H2a | `af37c4c` | 15,317,960 | 1,840 | `66801f665130d811` |
| H2b | `e4f4432` | 15,327,601 | 1,843 | `9933f0f07a3bd671` |
| H2 | `e69419c` | 16,253,582 | 1,949 | `403dd225e6d34d14` |
| H3 | `97652f1` | 16,311,386 | 1,974 | `d9b3ff0f76b467e2` |

The H2a, H2b and H2 archives were built **retroactively**, on 2026-09-01, after
the omission was noticed during H3: those three slices completed without the
packaging step. Each was packaged from its own close-out commit in a disposable
`git worktree`, so the live checkout was never touched and each archive reflects
the repository as it stood when that slice finished. Every archive passed
`scripts/check_release_zip.py`, and every tracked file in each was verified
byte-identical to its commit's tree — 1,365, 1,368, 1,376 and 1,974 files
respectively, zero mismatches.

H2a is packaged at `af37c4c` rather than `f4b8c27`, where its archive first
landed, because `be6cb8f` and `af37c4c` both revised H2a's own archive
afterwards. `af37c4c` is the state in which H2a's conclusions were final.

**One deliberate exclusion.** The `node_modules/.vite/vitest/...` cache file was
tracked from `e4f4432` until `8a28b5d`, so the H2b and H2 trees contain it and
their historical packaging would have failed `check_release_zip.py` — correctly.
It was removed from each worktree before packaging rather than the script being
patched, which leaves the historical validation running unchanged and produces
the archive each would have produced had the artifact never been committed. The
only difference between the historical `package_release.sh` and the current one
is the added `node_modules/*` exclusion; `check_release_zip.py` is byte-identical
at all four commits.

## v0.8 H3 completion handoff — 2026-09-01

H3 is complete and archived as
`plans/v0.8/007-installed-path-xdg-migration-purge-investigation.md`. It is an
**investigation**: no path default changed, no data moved, no Make lifecycle
target was added, nothing was deleted. Evidence is under `performance/v0.8-h3/`
and runs from `make validate`. H4 is next and remains unapproved.

Notrios has no path resolver — it has 25 places that each decide something
about location, and every defect worth reporting is a **disagreement between
two of them**:

- `internal/synckeys` asks `os.UserConfigDir()`, which refuses a relative
  `XDG_CONFIG_HOME` as the specification requires. `internal/profiles`
  hand-rolls the lookup and accepts it, resolving the registry against the
  working directory. With no `HOME` the registry becomes the bare relative path
  `.notrios/profiles.json`, so which database a `notrios://` link resolves to
  depends on the current directory.
- A generated profile puts the database, assets, projections, index and
  quarantine under the **config** root:
  `~/.config/notrios/profiles/<id>/data/notes.sqlite`. Observed against a real
  profile. This is the only consumer needing migration rather than a new
  default.
- `config/config.example.yaml` and `web/dist` are both resolved from the
  working directory ahead of the executable. The second matters most: it is a
  content-injection path into the application's own window, gated on where the
  user was standing when they launched it.
- Thirty-three derived-artifact sites are owner-only (20 directories `0700`,
  13 files `0600`); `EnsureDirectories` creates the primary roots `0755`. Two
  code paths create the *same* data directory with different modes —
  `EnsureDirectories` `0755`, `publish.File.Save` `0700` — so its permissions
  depend on which ran first. The encrypted backup of a user's notes is
  owner-only and the notes are world-readable.

**`PATH_CONSUMERS.json` will fail when H4 edits a consumer, and that is
deliberate.** Each of the 25 entries is anchored to a source substring that must
occur exactly once; `validate_evidence.py` re-checks them, so a changed consumer
breaks the inventory instead of leaving it describing code that no longer
exists. Update the entry as part of the change.

**`pathprobe/` asserts what is true today, including what H3 wants changed.**
Each such test names the H4 change that should break it and says so in its
failure message. When H4 lands, delete or invert them alongside the fix — the
failure is the reminder, not a regression. Flipping `EnsureDirectories` to
`0700` was tried and does exactly that.

Two classifications in `LAYOUT.json` are judgements, not conventions, and are
worth not silently reversing. **Quarantine is state, not cache** — it is the
record of what a note tried to fetch and the only copy of media a user may have
approved without localizing, and calling it cache would let `purge` delete it
without backup. **`XDG_RUNTIME_DIR` has no specified fallback**; inventing one
in `/tmp` would put staged plaintext backups somewhere world-traversable, so the
fallback is `<state>/runtime` at `0700`, and a runtime directory that exists but
is not owner-only is refused.

`test_backup_restore.py` executes the backup sequence rather than specifying
it: build, verify, delete *through the oracle*, restore, compare bytes and
modes. The ordering guarantee is the point — a truncated archive fails
verification, and nothing is deleted while verification fails.

The purge oracle decides containment on the **resolved** path so a symlink
cannot look contained while pointing out. Replacing `realpath` with textual
normalization was tried; the fixtures caught it, two of them via the separate
symlink rule. An unclassified backup category defaults to `backup_and_verify`,
so a root added later without a policy is never silently disposed of.

The two Python fixture files are `unittest`-based, not plain scripts: the other
`test_*.py` files here are run by `unittest discover`, and a module with no
`TestCase` would have been discovered, found empty, and reported as passing.

**Not verified:** no Windows or macOS execution (those matrix rows and the
`parentDir` separator defect are read from source); the mount-boundary rule is
modelled with an injected device lookup; the §5 failure models are designs, not
tests; no migration was performed; and the resolver model is Python, so H4's Go
implementation reproducing `RESOLUTION_TABLE.json` is not yet demonstrated.

## v0.8 H2 completion handoff — 2026-09-01

H2 is complete and archived as
`plans/v0.8/006-bounded-offline-mermaid-enablement.md`. **Mermaid is now
enabled.** `mermaid@11.17.2` is pinned and bundled; diagrams are rendered by
`web/src/mermaid-render.ts` in a post-pass over the preview, not by
`md-editor-rt`, which keeps `noMermaid: true`. H3 is next and remains
unapproved.

The fenced source is what is already on the page, so a diagram replaces it only
on success and every failure — malformed, oversized, timed out, sanitised to
nothing, or an internal crash — leaves the reader what they typed with a status
line saying why. Configured `securityLevel: 'strict'` with `htmlLabels: false`,
bounded at 20 diagrams per note, 65,536 source bytes, 500 edges, and 2,000 ms,
and the output is sanitised regardless of configuration.

**Two things a future change could easily re-break:**

`sanitizeDiagramSVG` removes media elements *before* stripping attributes.
Reversing that leaves an empty `<image>` behind, because the media pass judges
an element by where it points and the attribute pass has already removed it.

Mermaid drops a `notrios://` href before the sanitiser ever sees the SVG, while
keeping a remote `https` one. Note links are therefore reattached from the
diagram source — `click <node> "<uri>"`, only when the URI *parses* as a stable
link — and routed as `data-app-uri` with `href="#"`, the same in-app routing
every other note link uses. A DOMPurify hook does not reach mermaid's pass; this
was tried.

**Mermaid cannot render under jsdom.** The renderer is injectable and the unit
tests use a stub, so each fallback test distinguishes outcomes instead of
passing because everything fails. Real rendering is verified in Chromium under
the verbatim production CSP: zero violations, zero cross-origin requests, no
script execution. Evidence and the harness are under `performance/v0.8-h2/`.

The licence gate now carries the three H2a decisions, with the `khroma`
exemption pinned to a licence-file hash the gate re-verifies. npm packages went
251 to 361; the bundle 0.85 to 1.68 MB gzipped.

**Not verified:** no Wails webview smoke (the G18 contract asks for one before
desktop support is claimed), no worker cancellation, no screen-reader
assessment. Each is recorded in the archive.

## v0.8 H2b completion handoff — 2026-09-01

H2b is complete and archived as
`plans/v0.8/005-desktop-external-link-opening.md`. A remote `http`, `https`, or
`mailto:` link in a note now opens the system browser from the Wails desktop
window; previously the click was silently swallowed. The browser-tab path is
unchanged.

`web/src/desktop.ts` feature-detects `window.runtime.BrowserOpenURL` and admits
exactly the three schemes `normalizePreviewHTML` already keeps on an anchor.
`PreviewPane`'s existing click handler gained one branch and suppresses the
default only when the desktop shell took the link — a webview that followed the
link in place would replace the running application with a website, and a test
asserts `defaultPrevented` in both directions.

184 frontend tests, up from 172. Both guards were verified to fail against
deliberate regressions. The Go leg was verified by shadowing `xdg-open` on
`PATH`. **No click in a running GUI was observed**: the chain is proven at both
ends and read from Wails source in the middle. The archive records the manual
check to run on a machine with a display.

Worth knowing when debugging: the hand-off runs `xdg-open`, so the browser is
the desktop's configured URL handler and **not** `$BROWSER`, which `pkg/browser`
ignores on Linux.

Adding a section to `docs/gui.md` moved three pinned counts, each updated with
its reason: the docaudit expectation (201 to 202 manual sections), the G18a
grade baseline, and the G18f content hash for that one page.

This closes the dependency H2a recorded for its diagram link policy. H2 is next
and remains unapproved. No push, PR, merge, tag, release/upload,
evidence-reserve write, ISO, or physical burn was performed.

## v0.8 H2a completion handoff — 2026-09-01

H2a is complete and archived as
`plans/v0.8/004-mermaid-renderer-security-investigation.md`. It is an
investigation: **Mermaid remains disabled**, `web/package.json` and
`web/src/editor-assets.ts` are untouched, and every install happened in a
disposable directory outside `web/`. H2 is next and remains unapproved.

The recommendation is to enable Mermaid 11.17.2 under a specific containment
design. Measured in Chromium under the application's verbatim production CSP:
zero violations and no `unsafe-eval`, because the two `new Function` sites in
the dependency tree are unreachable from Mermaid and do not survive the Vite
bundle.

Mermaid's **default** configuration is not acceptable. A diagram label fetched a
remote image — the CSP permits `img-src https:` on purpose and the remote-media
policy covers Markdown images, not diagram labels — and every diagram emitted
the `foreignObject` the G18 contract asks to refuse. `htmlLabels: false` with
`securityLevel: 'strict'` gave zero `foreignObject`, zero cross-origin requests,
and no script execution. **One vector survives**: a `click` directive's remote
`href` stays in the SVG.

Link navigation is inverted by default, which is the finding most likely to be
re-broken. Under `strict`, a `notrios://` note link is **stripped** and a remote
`https://` link is **kept**; the cause is DOMPurify's default
`ALLOWED_URI_REGEXP`, which admits no custom scheme, not Mermaid's own URL
sanitiser. `loose` restores custom schemes but also re-admits `javascript:`
URLs into a clickable href. H2 must allowlist the `notrios` scheme, neutralise
remote hrefs, and resolve a note link through the existing in-app stable-link
resolver, so a diagram navigates to the user's own notes and a remote URL is
reached from the note that contains it.

Cost is roughly a doubled bundle (0.85 MB to about 1.75 MB gzipped), softened by
code splitting to 772 KiB uncompressed for one flowchart. Enablement also needs
three reviewed licence-gate decisions — `khroma` declares no licence in metadata
though it ships MIT, `dompurify` uses an SPDX `OR` expression the checker cannot
resolve, and `robust-predicates` is `Unlicense` — none a genuine licence
problem, all fail-closed today.

Approving H2 means accepting the bundle cost and those three decisions.

**Not validated, and recorded as such:** the 2000 ms render deadline was never
exercised by a genuinely slow successful render, because Mermaid's own
`maxEdges: 500` guard refuses large graphs before layout. The Wails v2 webview
smoke that the G18 contract requires before `noMermaid` changes, a worker
cancellation prototype, and an accessibility review also remain undone.

H2a also found a defect outside its own scope, now tracked as `PLAN.md` **H2b**:
a remote link in a note opens a new tab in the loopback web UI but **does
nothing in the Wails desktop window**. Wails v2.13.0's Linux webview connects
neither the `create` nor the `decide-policy` signal, so a `target="_blank"`
click is swallowed by WebKitGTK's default handler, and the repository makes no
`BrowserOpenURL` call. The mechanism exists —
`window.runtime.BrowserOpenURL(url)` is in the desktop JS runtime — so the fix
is small. This was traced through Wails source, not observed in a running GUI.
H2's link policy depends on it: de-linking a remote URL in a diagram assumes the
reader can reach it from the note, which on the desktop they currently cannot.
H2b is independent of Mermaid and must not wait on H2 approval.

Evidence is under `performance/v0.8-h2a/`;
`python3 performance/v0.8-h2a/validate_evidence.py` is in the scaffold gate and
was verified to fail both on a falsified CSP claim and on a quietly deleted
caveat. No push, PR, merge, tag, release/upload, evidence-reserve write, ISO, or
physical burn was performed.

## v0.8 H1 completion handoff — 2026-09-01

H1 is complete and archived as
`plans/v0.8/003-shared-application-facade-abi-library.md`. It was delivered in
four separately committed slices so an interruption could not land mid-rewrite:
`7810dc9` facade, `e1ba989` REST migration, `ba18209` SQLite vendoring, and
`623814a` plus `85ce573` the C ABI. H2a is next and remains unapproved.

`internal/application` is now the transport-neutral application contract, with a
`Kind`/`Error` model, a narrow eleven-method `Repository` seam, and an AST guard
that fails on any store-typed export other than that seam. Nineteen REST call
sites in `server.go` and `noteops.go` use it; response parity is enforced by a
test that drives the old and new error writers with the same errors and requires
byte-identical output.

The store now statically links the vendored SQLite 3.53.4 amalgamation under
`internal/store/csqlite/` with hidden visibility. **`libsqlite3-dev` and
`pkg-config` are no longer needed to build**, and a cold `internal/store` build
takes roughly 3m40s while 9.5 MB of C compiles. `python3
scripts/check_sqlite_provenance.py` is in the scaffold gate and enforces the
hashes, the compile options, and the absence of any system SQLite include.

`cmd/notrioslib` builds as `c-shared` and `c-archive` and exports exactly the 12
frozen ABI symbols with zero exported `sqlite3_*` and no dynamic SQLite. `make
abi` builds it and runs the C host acceptance test.

Deliberately unchanged, each with its reason recorded in code: the MCP adapter
still calls the store directly; `handleResourceContent` stays on the store
because the facade's bounded stream hides the `io.ReadSeeker` that HTTP Range
needs; and `searchMerged` stays because moving it would make the facade own the
Recoll sidecar. Android was not exercised — H0's API-35 evidence stands and H11
owns the emulator run.

Separately, the Claude side of the agent-usage preflight was repaired (`17939bb`).
Claude Code writes no usage file; it hands rate limits to the configured
`statusLine` command. `scripts/claude_statusline_usage.py` must be installed as
that command in `~/.claude/settings.json` or the probe reports `unknown` and
cannot gate a long run. A cache older than 30 minutes, or one whose windows are
all past `resets_at`, reports `stale` and is treated like `unknown`.

The H1 close-out and clean packaging commit is `fa3803c`. The verified local
source snapshot is `dist/notrios-v0.8-h1-fa3803c.zip`: 15,269,890 bytes, 1,815
entries, SHA-256
`4b156a19cf500031676b377545435e8966e3eabff7f2a1baa0bd0607cbf0b538`. It was
copied byte-for-byte to
`/home/renes/evidence/notrios/notrios-v0.8-h1-fa3803c.zip`; source and
destination hashes, sizes, and a `cmp` all match.

`scripts/package_release.sh` reran the full gate set and
`python3 scripts/check_release_zip.py` passed independently. An independent
member check found `web/dist/index.html`, the H1 plan archive, the vendored
`csqlite/` sources with `NOTICE` and `PROVENANCE.json`, the cgo build owner and
shim, `cmd/notrioslib` with its frozen header and C host test,
`internal/application`, `internal/abi`, both new scripts, and `PLAN.md`, with
zero `.git`, `node_modules`, runtime data, or SQLite database entries. The
vendored `sqlite3.c` and `sqlite3.h` inside the ZIP still hash to the pinned
`b1dd5d74…b28189` and `919e7f2e…5910e1d`.

The snapshot grew from roughly 5.3 MB to 15.3 MB because the vendored
amalgamation is now tracked source. No push, PR, merge, tag, release/upload,
evidence-reserve write, ISO, or physical burn was performed.

## v0.8 H0 completion handoff — 2026-08-31

H0 is complete and archived as
`plans/v0.8/002-application-facade-c-abi-sqlite-ownership-investigation.md`.
It selects a new `internal/application` facade owner, official SQLite 3.53.4
amalgamation with exact SHA3/SHA-256 provenance and hidden static cgo linkage,
the frozen ABI-major-1 12-symbol polling/stream contract, and API-35 x86_64 as
the only runtime-qualified Android target. Android arm64-v8a is build-only.

The unchanged schema-v27 store/snapshot/sync code passed against the pin on
Linux and the API-35 emulator. A real-store shared library bootstrapped schema
v27 with no dynamic SQLite dependency or exported `sqlite3_*` symbol. The
separate ABI harness passed c-shared/c-archive headers, exact symbols, C-memory
ownership, generation handles, cancel/poll/events, streams, and concurrent
shutdown on Linux and Android. A checkpointed desktop-C database crossed to
Android modernc 3.53.3 and back to desktop C with matching pulled SHA-256,
integrity, WAL, 1,001 rows, and the exact new row.

`performance/v0.8-h0/REPORT.json` is the machine-checked decision and cost
record; its validator and disposable source/facade/ABI/store-link/SQLite probes
live beside it. No downloaded amalgamation, binary, database, cache, or private
content is tracked. The emulator directory was removed, the emulator stopped,
and no H0 process remains. H1 is next and separately approval-gated. No push,
PR, merge, tag, release/upload, evidence-reserve write, ISO, or physical burn
was performed.

The H0 implementation/evidence commit and clean packaging source is `e164aeb`.
The verified local source snapshot is
`dist/notrios-v0.8-h0-e164aeb.zip`: 5,334,539 bytes, 1,778 entries, SHA-256
`5b131883de2376bdeb6ec809742524c237b3e0bbd6a5621b501903ff28b9bf2f`.
The release checker and independent member check found `web/dist/index.html`,
`performance/v0.8-h0/REPORT.json`, and the H0 archive; packaging exclusions
cover `.git`, `node_modules`, runtime data, and SQLite databases.

## v0.8 installation planning handoff — 2026-08-31

The unstarted v0.8 plan now has H0-H13 independently approvable items. The
installation amendment is archived at
`plans/v0.8/001-installation-delivery-plan-amendment.md`; it adds a central
installed/XDG path investigation and implementation, safe end-user-location
Make lifecycle, Ubuntu-priority packaging, evidence-gated Windows/macOS native
GitHub jobs, installed integration, and delayed `develop`-to-`main` PR and
branch synchronization. `ROADMAP.md` now carries the installation path through
v0.9 hardening to a v1.0 GitHub download that requires no source repository or
developer toolchain.

The safety contract is explicit: uninstall removes only manifest-owned
installed artifacts and leaves user state; purge backs up and verifies
config/data/state before bounded deletion by default; `DRYRUN=1` is an exact
zero-mutation preview; `FORCE=1` suppresses only the prompt; and
`NO_BACKUP=1` takes the deep-warning path and skips backup. External profile
paths are not automatically deleted. `clean`/`clobber` remain development-only.

Read-only branch checks found remote
`main=265ef4ef84ea90f0e325522a3a4308a5804f122c`, remote
`develop=26b0925c21b3ecc264c370936e42d4b973548b8d`, and pre-amendment local
`develop=fd2192d1e830833fcf74b191bfc01851a61ed8bd`. Remote `main` is an
ancestor of local `develop`, which was 133 commits ahead of remote `develop`.
H12 must re-fetch/recheck and delays the first v0.8 push until native GitHub
runners are needed. No push, PR, merge, tag, release, installer upload,
reserve/ISO write, or burn occurred.

The planning commit is `9c9e511`; the clean packaging checkpoint is `6140292`.
The verified local source snapshot is
`dist/notrios-v0.8-installation-plan-6140292.zip`: 5,283,894 bytes, 1,742
entries, SHA-256
`e47bd6622e97fcab0e08f94e55fc04680867907f14dd0f3f3f9a2655b1e22818`.
Independent inventory found `PLAN.md`, `ROADMAP.md`, `web/dist/index.html`, and
the v0.8 planning archive, with no `.git`, `node_modules`, runtime `data`, or
SQLite database entries.

This was planning only. No lifecycle target, runtime path, migration, installer,
or workflow was implemented. H0 remains next and unapproved.

## G20 completion handoff — 2026-08-31

G20 is complete and archived as
`plans/v0.7/043-full-convergence-release-wrap-up.md`; the complete milestone
plan is `plans/v0.7/000-v0.7-plan.md`. Product version is 0.7.0 and canonical
schema is v27 (migrations v19-v27). `PLAN.md` now contains the unstarted v0.8
H0-H13 plan; H0 is next and remains unapproved.

The exact convergence, directory/REST process, catch-up/reset/retention,
lazy-resource, conflict/fault, abuse, and fresh/upgrade selectors pass. The
focused G20 gate validates the complete service/httpapi/archive/carrier suites,
37 exact Go-module and 251 exact npm-package licenses, and the frozen
aggregate-only G8/G14e/G17 evidence without rereading the private corpus.

The standard security scan completed all seven enumerated surfaces and
validated six findings. G20 centralizes TLS selection and finite remote
peer-route admission; enforces loopback/local-Host ordinary access,
same-origin/Wails browser mutations, exactly-one 8 MiB JSON/MCP bodies, and the
16 GiB resource ceiling; and roots/bounds carrier plus legacy-archive access.
One allowed static bypass review found five narrower gaps, all closed in one
correction cycle. The exact scan hashes, dispositions, residual limitations,
and derived structural-hardening portfolio are under
`performance/v0.7-g20/`.

Implementation commit `c1127f1` and durable packaging checkpoint `4bc2577` are
complete. The verified release snapshot is
`dist/notrios-v0.7-g20-4bc2577.zip`: 5,270,098 bytes, 1,740 entries, and SHA-256
`0710217d9fa98dff34ac7d9f0e697fffd7d956230cd95506211c8a2b0660513d`.
It was copied byte-for-byte to
`/home/renes/evidence/notrios/notrios-v0.7-g20-4bc2577.zip`; source and
destination hashes and sizes match. Independent inventory verification found
`web/dist/index.html`, the G20 report/security/hardening evidence, both v0.7
plan archives, and the new unstarted v0.8 `PLAN.md`, with no `.git`,
`node_modules`, runtime data, or SQLite entries. No push, tag, public release,
reserve/ISO write, or physical burn was performed or authorized.

## G19 completion handoff — 2026-08-30

G19 is complete and archived as
`plans/v0.7/042-archive-v2-compatibility-bridge.md`. The external consumer
contract under `contracts/archive-v2/` now contains a production-derived
capability/version/limit registry, five strict Draft 2020-12 schemas, three
complete independently generated and fully verified loose/packed/schema-27
goldens, current/previous reader matrix probes, a production-shaped manifest-
only physical refusal, and byte-identical separately labelled G9 NCB1/NEV1
vectors. The pinned docs builder publishes all 27 files byte-for-byte at
`/notrios/contracts/archive-v2/`.

`notriosctl compatibility archive-v2 [--reader
current-v2|previous-loose-v2] <archive-dir|manifest.json>` performs bounded
declaration-only admission and emits structured JSON. Acceptance exits 0 and
still requires `verify archive-v2`; format/capability refusal exits 1; usage
errors exit 2. It refuses physical SQLite images without opening payloads and
trusts only the exact `notrios-archive-v2` fallback declaration.

Focused G19, full Go, vet, docs-generation/audit/site, scaffold, web typecheck,
172 frontend tests, production builds, and dependency-audit gates pass. The
local MoveNotes audit found no consumer or `notrios2sql.py`, so no external
repository was modified and no cross-repository test is claimed. G20 has since
completed and closed v0.7.

The implementation commit is `0ec2803`. The verified release snapshot is
`dist/notrios-v0.7-g19-0ec2803.zip`: 5,196,767 bytes, 1,709 entries, and
SHA-256
`ba5a004a1a7d46860360038caa9fd9223362a6de81620905216f97d8d3c71396`.
Independent inventory verification found `web/dist/`, the published contract,
G19 report, and plan archive, with zero `.git`, `node_modules`, runtime data,
or SQLite entries. No push, evidence-reserve write, ISO, or physical burn was
authorized or performed.

## G18g completion handoff — 2026-08-30

G18g implementation and focused acceptance are complete. `docs-site/` contains
the exact G18b-selected 44-file Ledger snapshot and provenance; the production
builder/Pages/CI/release path pins Hugo Extended 0.164.0, Node 26.3.0, and
Pagefind 1.5.2. It preserves 15 routes, 199 G18a sections, two aliases, 15
Pagefind-scoped articles, local-only runtime assets, and raw Help bytes/IDs.
The publication-only CLI adapter preserves angle placeholders without mutating
`docs/` or loosening Goldmark safety.

Focused source/site/mutation/Help checks pass. The Browser plugin was absent,
so documented Playwright/Chrome fallback passed 1440×960 and 390×844 rendered
QA with zero console/page/HTTP/CSP/external-request problems. A measured build
took 2.71 seconds at 82,432 KiB and emitted 53 files/1,419,897 bytes, indexing
15 pages/3,438 words. Repeat Hugo output is byte-identical; Pagefind retains
the G18b-qualified hashed-shard variance with identical scope/search behavior.
Evidence is under `performance/v0.7-g18g/`; the plan archive is
`plans/v0.7/041-hugo-ledger-production-site.md`.

The implementation commit is `8d81eb3`. The verified release snapshot is
`dist/notrios-v0.7-g18g-8d81eb3.zip`: 5,137,885 bytes, 1,635 entries, and
SHA-256
`c26e1bc16657f16842bb44c14ca959ed0b582481bb50ca32f6ca07cb0f80e9ff`.
Independent inventory verification found `web/dist/`, all 44 theme files, the
G18g report and plan archive, with zero `.git`, `node_modules`, runtime data,
or SQLite entries. No temporary docs server remains. G19 is next and remains
unapproved; no GitHub push, reserve write, ISO, or physical burn is authorized.

## G18f completion handoff — 2026-08-30

G18f is complete and archived as
`plans/v0.7/040-generated-docs-advisory-review.md`. `docgen --user/--api` now
uses explicit templates to place 15 separate source-adjacent fragments into
five committed Markdown pages. Closed adapters generate 412 finite config,
CLI, REST/OpenAPI, MCP, and GUI rows. `TestDocsAreCurrent`, registry parity,
and `make g18f-validate` are deterministic CI/release gates over the same
Markdown consumed by the site and Help seeder.

Maintainer-only `doccheck` accepts only loopback HTTP, strips source-adjacent
claims from bounded declaration/callee views, explains source before revealing
the claim, repeats each review, and records prompt/source hashes, model, tokens,
timing, variance, and zero API cost. Its decomposed Qwen 2.5 Coder 1.5B run
scored 7/16 calibration decisions and contradicted 11/13 fragments. Every
contradiction has a reviewed disposition tied to deterministic generation or a
registered claim test. All 26 prose-only action attempts failed exact G18d/G18e
fixture matching and were rejected; no model output ran or rewrote prose.

The independently known schema-v20 prose and CLI sample were corrected to the
canonical v27. Evidence is under `performance/v0.7-g18f/`. The implementation
commit is `c11277e`. The verified release snapshot is
`dist/notrios-v0.7-g18f-c11277e.zip`: 5,029,408 bytes, 1,546 entries, and
SHA-256
`c0f8249ab6595f2e0ad3249aaa7c773b9c9eb19c599abed4c29f9ce1505f3e11`.
Independent inventory verification found the built UI, G18f deterministic and
advisory reports, triage, and plan archive, with no `.git`, `web/node_modules`,
runtime `data`, or SQLite entries.

G18g is the next incomplete item and remains unapproved. No external model,
private corpus, note/database content, reserve write, GitHub push, ISO, or
physical burn was used.

## G18e completion handoff — 2026-08-29

G18e is complete and archived as
`plans/v0.7/039-executed-gui-journeys.md`. A strict 37-entry GUI-procedure
manifest resolves the frozen G18a Go/TypeScript source anchors. Thirty-two
journeys execute as 44 real browser results across desktop 1440×960 and narrow
Sync Center 390×844; five remain explicitly unverified under finite owner and
reason records. The Browser plugin was absent, so the checked report records
the approved deterministic Playwright fallback against disposable loopback
host/joiner daemons.

The implementation commit is `91dec2c`. The verified handoff archive is
`dist/notrios-v0.7-g18e-91dec2c.zip`, copied byte-for-byte to
`/home/renes/evidence/notrios/notrios-v0.7-g18e-91dec2c.zip`. It is 4,961,593
bytes with 1,518 entries and SHA-256
`596c7b019863476a0aad84dc82e91351a4b31a3336f8f7d70d146ae6af638c40`.
Independent inventory verification found the built UI, G18e report, and G18e
plan archive, with zero `.git`, `web/node_modules`, or SQLite entries.

Every pass contains user-action metrics plus visible and canonical/API
postconditions. The aggregate is 162 clicks, 34 keypresses, 40 typed-field
occurrences, eight branches, modal depth one, and seven recovery steps after
fixture/accessibility mechanics are excluded. Browser health has zero
unexpected console warning/error, page error, actual CSP violation, external
request, journey failure, or health failure. The exact recovered 401 from the
deliberate wrong-password branch remains visible in raw health evidence.

Execution found and fixed two existing contract defects: the Sync Center close
target is now 44 by 44 CSS pixels, and remote preview images remain inert until
the server-side quarantine/SSRF media path localizes them. Documentation now
states honestly that the current GUI does not expose import/export job control
or v0.6's batch organizer API. Transient reviewed screenshots are
`/tmp/notrios-g18e-desktop.png` and
`/tmp/notrios-g18e-narrow-sync.png`; they are not committed or used as result
assertions. G18f has since completed; G18g is next and remains unapproved.

## G18d release handoff complete

G18d implementation and its archive are committed on `develop` as `a3c0151`;
the durable packaging-resume checkpoint is `a8692d3`. After the active Codex
window reset, the mandatory usage preflight passed and
`scripts/package_release.sh` produced
`dist/notrios-v0.7-g18d-a3c0151.zip` from that clean checkpoint. The archive is
4,918,236 bytes (1,501 entries) with SHA-256
`b0cc8029bf497b7cd986836151ba55002b7c219b47fed4a6550e1324a6b74db4`.

The packager reran the complete Go/scaffold/frontend/documentation evidence
chain and its ZIP verifier. An independent verification also found the built
UI, G18d report, and G18d plan archive present, with no `.git`,
`web/node_modules`, runtime `data`, or SQLite entries. G18e has since completed;
G18f has since completed; G18g is next and remains unapproved.

Current phase: v0.1 through **v0.7** are complete; product version is **0.7.0**
and the schema is **v27**. The eight v0.6 slices are archived under
`plans/v0.6/`: F0 (notebook targeting, landed with v0.5 E10), F1 (batch
organizer transactions), F2 (MCP tool scopes), F3 (MCP read coverage and HTTP
`Range`), F4 (note templates and task extraction), F5 (graph views that stay
readable at scale), F6 (job control plane), and F7 (documentation and release
wrap-up). The thirteen v0.5 slices remain archived under `plans/v0.5/`.

The completed v0.7 plan is archived as `plans/v0.7/000-v0.7-plan.md`.
`PLAN.md` now holds the **unstarted v0.8 installation, configuration,
shared-core, and portability plan** with H0-H13 independently approvable items.
The historical v0.7 plan contained
independently approvable G0-G20 slices plus the newly inserted, blocking
G14a-G14e archive-scalability sequence, the newly inserted blocking G17a-G17b
evidence-preservation sequence, and the newly planned G18a-G18g documentation-
integrity/Hugo-Ledger sequence and the G18c.1 agent-workflow amendment. The user's 2026-08-11
review resolved the policy decisions through G17, including mandatory payload
encryption and per-replica Ed25519 signatures. **G0-G18f are complete** and
archived under `plans/v0.7/`; their reviewed evidence is under
`performance/v0.7-g0/`, `performance/v0.7-g1/`, `performance/v0.7-g1a/`,
`performance/v0.7-g2/`, `performance/v0.7-g4/`, `performance/v0.7-g5/`,
`performance/v0.7-g6/`, `performance/v0.7-g7/`, `performance/v0.7-g8/`,
`performance/v0.7-g9/`, `performance/v0.7-g10/`, `performance/v0.7-g11/`, and
`performance/v0.7-g12/`, `performance/v0.7-g13/`, `performance/v0.7-g14/`,
`performance/v0.7-g14a/`, `performance/v0.7-g14b/`, and
`performance/v0.7-g14c/`, `performance/v0.7-g14d/`, and
`performance/v0.7-g14e/`, `performance/v0.7-g16/`, and
`performance/v0.7-g17/`, `performance/v0.7-g18/`, and
`performance/v0.7-g18a/`, `performance/v0.7-g18b/`,
`performance/v0.7-g18c/`, `performance/v0.7-g18d/`, and
`performance/v0.7-g18e/`. G15's archive is
`plans/v0.7/024-durable-sync-jobs.md`; G16's archive is
`plans/v0.7/025-sync-recovery-ui.md`; G17's archive is
`plans/v0.7/026-peer-retention-gc-repair.md`; G17a and its later decision record
are `plans/v0.7/028-evidence-preservation-contract.md` and
`plans/v0.7/029-evidence-handling-decisions.md`. G17b is complete and archived
as `plans/v0.7/030-evidence-seals-iso-reserve.md`. G18 is complete and archived
as `plans/v0.7/031-shared-core-ffi-portability-handoff.md`. G18a is complete and
archived as `plans/v0.7/032-documentation-anchor-investigation.md`. The G18
modernc evaluation amendment is `plans/v0.7/033-modernc-sqlite-evaluation.md`.
The Android-emulator runtime amendment is
`plans/v0.7/034-android-emulator-modernc-runtime.md`.
G18b is archived as
`plans/v0.7/035-hugo-ledger-reproducible-site-contract.md`.
G18c is archived as `plans/v0.7/036-documentation-anchor-audit.md`.
G18c.1 is archived as `plans/v0.7/037-agent-usage-preflight.md`.
G18d is archived as `plans/v0.7/038-executed-documentation-examples.md`.
G18e is archived as `plans/v0.7/039-executed-gui-journeys.md`.
**G0-G18f are complete.
The G17b pre-push verifier is mandatory before any future GitHub push; no push
or physical burn was authorized or performed. G18g-G20 have since completed;
v0.8 H0 is next and unapproved.** G14b selected option B: a
required compatible same-schema SQLite-image plus bounded packed-assets
capability for whole-library full backup/catch-up, retaining packed semantic
archive-v2 for subset, merge, schema-independent interchange, and fallback.
G14c implements that exact representation, G14d integrates it, and G14e passed
the full-scale acceptance matrix and froze the format.

G18b is investigation-only and changes no production documentation pipeline.
It selected a direct minimal vendor for G18g: exact Ledger commit
`f9d28ea297427890ecffa31fa74caa9ee385d9f5`, 44 files/173,947 bytes, LICENSE,
and deterministic provenance under `performance/v0.7-g18b/prototype/`.
The clean Hugo 0.164/Pagefind 1.5.2 build byte-copies all 15 `docs/` sources,
preserves every `.html` route and all 199 G18a section IDs through five heading
mappings/two aliases, indexes 15 pages/3,191 words, and passes the `Argon2id`,
keyboard, mobile, sampled-contrast, `/notrios/`, and zero-third-party-request
smokes. The present Marked command already breaks 32 fragments because it emits
no heading IDs. Submodule archives omit the theme payload; initial Hugo Module
resolution is network/cache dependent and its generated vendor tree omitted
LICENSE. Repeat Hugo output is byte-identical; Pagefind varies hashed shard
names while preserving scope/search, so only semantic reproducibility is
claimed. G18f is now complete; G18g is the next incomplete item and remains
unapproved.

G18c implements a repository-only `docaudit` command/library over the G18a
contract. It parses the Go-1.25-compatible no-space directive grammar, resolves
exact Go and TypeScript declarations, binds fragments to the 15-page/199-
section Markdown template, validates four typed claim-to-test edges, and
accounts bidirectionally for 131 executable-shaped fences and nine proposed GUI
journeys. The first 12 fragments cover version/schema, config keys/defaults,
CLI usage, REST registrations, MCP tools and ordinary/sync scopes, and exact
REST/GUI destructive confirmations. The checked 351-unit report is 0 executed,
8 generated, 4 claimed, and 339 unverified. Existing manual sections—including
the preserved schema-v20 drift—stay independently unverified. Twenty Go graph
mutations plus eight TypeScript cases gate dangling, duplicate, orphan,
audience, and source errors; exact App/EditorPane/SyncCenter regressions protect
the installed TypeScript 7 fallback. CI and release ZIP assembly validate the
exact report.

G18c.1 adds developer-workflow protection without changing the product.
`scripts/check_agent_usage.py` reads Codex rolling windows through the local,
model-free app-server method and reads Claude only from an explicit local cache
when Claude is actually running. It never calls `codex exec /status` or
`claude -p`, and missing telemetry is `unknown`, never 100%. Claude Code writes
no usage file of its own, so that cache is produced by installing
`scripts/claude_statusline_usage.py` as the `statusLine` command in
`~/.claude/settings.json`; without it the Claude side is permanently `unknown`. The shared wrapper
is advisory by default, can be strict or disabled, and runs before G14b/G14e
durable phase starts and repeated long profiles. Optional before/after history
must stay outside canonical evidence and is isolated by agent, model, effort,
operation, run, duration, and reset window. The 20% floor is raised only by the
largest matching observed drop plus five points. At session start, run the
fixture suite and a live probe as described in `AGENTS.md`; if a client update
changes the contract, fix fixtures before relying on strict mode. G18e is
complete and G18f has since completed; G18g is the next incomplete, unapproved item.

G18d upgrades the strict 131-entry non-GUI example registry to schema v2. The
real-binary scratch harness executes 63 literal CLI, configuration, REST, and
MCP examples and leaves 68 entries under six finite reviewed reasons; no
provisional reason remains. Requests and responses validate against the shipped
OpenAPI contract, configuration fragments use the production loader, MCP calls
check schema/scope, and CLI postconditions inspect real SQLite/archive state.
Minimal synthetic import fixtures replace private exports. The checked report
and mutation matrix live under `performance/v0.7-g18d/`; its validator reruns
the entire manifest and owns current execution freshness while the G18c report
remains the frozen pre-execution baseline. No external network, private corpus,
push, reserve write, ISO, or burn occurred.

G15 advances schema v26 with a sync-only durable outbox over the F6 job
records. `internal/syncjobs/` drains only explicitly queued/due rows; it never
creates periodic work. Atomic target leases, content-free phase checkpoints,
heartbeats, cancellation, 64 MiB default/16 GiB maximum per-attempt artifact
budgets, stale-worker recovery, and deterministic jittered retry are live over
the existing directory/REST round. Targets persist only as opaque digests.
CLI adds `sync start` and `jobs retry [--reset]`; local REST adds bounded
plan/start/status/retry/reset/conflict routes. MCP has an orthogonal
`mcp.sync_scope=disabled|status|control`, default disabled, and can control only
its own path-free incremental/resource jobs—never locations, credentials,
keys, bulk bytes, enrollment, backup/restore, retirement, purge,
catch-up/restore-prep, reset, or another actor's job. The full Go/UI/docs/smoke
validation passed; OpenAPI and code now match all 93 registered operations.

G16 adds no schema. The responsive loopback/native Sync Center names the active
profile/database/replica; configures none/directory/REST with a native-only
folder chooser and explicit inbound switch; discovers, invites, pairs, and
separately grants complete-snapshot permission; shows durable job/peer/resource/
repair state; resolves body conflicts with an explicit two-parent revision; and
stages catch-up/reset for review. NPB1 wraps the verified physical NBK1 payload
key with an Argon2id password and inspection always leaves canonical state
untouched. Passwords are cleared and never stored. The warned `0600` provider
is injectable and shared live by pairing, the peer surface, and each job attempt.
The real two-daemon acceptance flow and regular Playwright desktop/mobile sweep
passed; no Browser plugin or mobile build is claimed. OpenAPI and code match all
106 normalized non-HEAD operations.

G17 advances schema v27. `internal/store/sync_retention.go` plans operation and
tombstone collection at the minimum of the configurable 90-day age floor, a
verified physical-snapshot vector, every active peer acknowledgement, and the
current vector; floors only advance. `sync retention --snapshot <dir>` fully
re-verifies the retained image and database identity before dry-run or apply,
and apply needs the exact digest. Sync-aware `gc` likewise requires a currently
verified snapshot. Signed ordinary `replica.retire` operations revoke old peer
credentials, propagate without requiring every peer online, expose peers that
have not acknowledged the decision, and prevent stale re-enrollment. Key
revocation alone deliberately keeps the acknowledgement watermark open. Signed
purge retains payload until safe and preserves compact permanent-death identity
afterward. Peers below a collected floor get typed, non-automatic snapshot
catch-up; repair selects only a retained snapshot covering every existing
floor, then newer log operations. The Sync Center is a path-free review surface
with explicit retirement confirmation and no retention-apply HTTP route.
OpenAPI/code parity is 109 operations. Generated full-corpus-scale evidence
measured 169,906,176 incremental bytes for 382,206 representative operations,
so the reviewed 90-day default remains.

G18 completes the v0.7 portability handoff without implementing future product
work. Machine-checked evidence under `performance/v0.7-g18/` reconciles 109 API
operations and 19 runtime/platform capabilities. It identifies useful Store,
provider, context, job, and bounded-reader seams while recording that Service
still owns HTTP and no framework-neutral facade exists yet. The ABI-major-1
proposal uses 12 version/lifecycle/dispatch/cancel/poll/event/stream/release
symbols, generation-bearing opaque handles, a closed error taxonomy, explicit
output release, and 1 MiB JSON/stream-read bounds. Linux Go shared/archive
build modes work against host SQLite. Debian's host `sqlite3.h` and x86-64
library are installed, but NDK compilation still stops at the absent Android-
target SQLite boundary; adding `/usr/include` proves invalid because it mixes
glibc host headers into the Android sysroot. A 2026-08-26 follow-up confirms
that SQLite is not a public NDK C API and Jetpack `BundledSQLiteDriver` is a
Kotlin/Room driver, not a drop-in `sqlite3.h`/cgo dependency. v0.8 H0 now
compares a checksum-pinned upstream amalgamation control with exact
`modernc.org/sqlite`/`modernc.org/libc` pins in the Go shared core. The modernc
probe passed Linux FTS5/JSON/WAL and a C-modernc-C format round trip; it
cross-built Android executable and NDK `c-shared` artifacts, macOS, and Windows.
The exact candidate then ran on an API 35 x86_64 AVD: SQLite 3.53.3,
FTS5/JSON/WAL, integrity, close/reopen, emulator-reboot persistence, and NDK
`c-shared` `dlopen`/`dlclose` all passed. This is an H0 pre-gate, not upstream
Android support or a product selection. Android arm64 and the real ABI/store/
snapshot/sync workload remain untested; iOS needs an Apple link host, and
`js/wasm` failed in modernc libc. Exact package/update/options,
ABI/minSdk, driver semantics/performance, one-engine policy, sandbox/WAL
lifecycle, and desktop/Android interoperability remain blocking. Flutter Doctor
now reports no issues
after `/usr/bin/clang` and `/usr/bin/clang++` PATH resolution was restored;
Swiftly is reachable but has no selected Swift toolchain. Two AVDs are now
configured; the API 35 test emulator was stopped cleanly and no device remains
connected. There is still no Flutter/Android app build or device-support claim.
Rendered browser QA also confirms that the current editable md-editor-rt/
CodeMirror UI exposes case, regexp, by-word, and replacement controls through
Ctrl+F; Ctrl/Cmd+H is not a default binding.
Mermaid remains disabled behind `noMermaid: true`; exact browser/Wails/offline/
CSP/sanitization/accessibility and size/time/heap gates now belong to v0.8.

G18a freezes the documentation-integrity premise without moving prose. Its
checked inventory covers the 15 public/Help pages as 199 non-fenced H1-H3 migration units,
plus 55 CLI forms, 59 config keys, 109 OpenAPI operations, 46 MCP tools, zero MCP
protocol resources, and nine proposed GUI journeys. Every current section is
honestly `unverified` until source-adjacent prose names a generated registry,
executable check, or executed result. Go and TS/TSX anchors resolve named
declarations and a bounded one-hop direct-callee slice; path/line, missing, and
ambiguous anchors fail. One doc group carries one user/API/maintainer audience,
while rationale is unmarked. Go 1.27 confirms directive stripping from
`CommentGroup.Text`, but the Go 1.25 minimum predates `ast.ParseDirective`, so
G18c uses the tested compatibility parser. The eight-case calibration set
preserves the real schema-v20/v27 contradiction, a one-word negation pair,
rationale, and scope limits. No external model was called and semantic review
remains a future advisory, never a CI gate.

The 2026-08-24 planning amendment adds G18a-G18g between the portability
handoff and G19 compatibility bridge. Two investigations first freeze a
cross-language source-anchor/claim grammar and a reproducible
`hugo-theme-ledger` integration. The implementation slices then add an honest
executed/generated/claimed/unverified audit, result-bearing CLI/config/REST/MCP
examples, browser-executed GUI journeys with action-length evidence, generated
user/API fragments with freshness checks, calibrated advisory blind-code
contradiction/actionability review, and the pinned Hugo/Ledger+Pagefind site.
Semantic similarity is explicitly rejected for truth checking; model output is
never a CI gate. Raw `docs/` Markdown remains the one source for protected
offline Help. Planning inventory found `docs/service.md`'s stale schema-v20
claim versus canonical v27 and preserves it as G18a calibration evidence before
the audited correction. No new slice is approved by this amendment.

The 2026-08-24 evidence-preservation amendment inserts G17a-G17b ahead of G18
and any GitHub push. G17a is complete. Its exact read-only top-level capture
contains 78 files (74 ZIPs and four PNGs), 270,506,844 source bytes, no existing
signature/timestamp sidecars, and 73 distinct six-anchor exact commit mappings;
legacy `notrios.zip` remains unknown. All ZIP and PNG structural checks pass.
The same root has three recursive G14 private benchmark workspaces; an all-
recursive ISO would violate the private-data boundary. The user selected curated
top-level handoffs—including the G17a and any verified pre-G17b decision ZIPs—
and excluded every recursive workspace. The curated G17a sources print as a
270,962,688-byte ISO, 39.75% of the 650 MiB project budget.

Generated `/tmp` fixtures proved canonical chain mutation refusal, detached
OpenPGP verification, RFC 3161 nonce/imprint/policy and explicit-CA checks with
wrong-data/wrong-CA refusal, and two byte-identical xorriso builds plus exact
extraction. `EVIDENCE_PRESERVATION.md` selects one signature per artifact and
one RFC 3161 token over the signed batch checkpoint that hashes all artifacts
and signatures. The selected evidence identity is UID `Rene Sugar (Evidence
Identity) <rene.sugar@gmail.com>`, primary fingerprint
`AEE5F82F2C216D6D15992C8DC96A1C6039BC8098`, and exact Ed25519 signing subkey
`4ABEB98AF99C8321931BCF282C6A8A4568264005`, expiring 2027-08-25. Read-only
metadata shows `sec#`/usable `ssb`; the user attests that separate Secret Service
items hold the operational subkey export and passphrase. Never retrieve or log
them during planning, and force the subkey with its fingerprint plus `!` during
authorized signing. DigiCert `http://timestamp.digicert.com` is primary and
Sectigo `http://timestamp.sectigo.com` fallback; an authorized generated pilot
must still pin and verify the actual policy OID and responder chain before any
production request.
G17b backfilled without rewriting originals, checked the chained manifest and
outer ISO catalog into Git, and issued immutable volume `NTR-EV-0001` only under
`/media/renes/SEAGATE2TB/notrios-evidence/`. The 284,932,096-byte ISO SHA-256 is
`f4df1e047e3f372efdf5ab3d3a89089f2413c1e91243afaff01012054161258f`;
the final outer catalog SHA-256 is
`b47f9a7d1879459ee7b0c269aafe852c577e514fadf3369ba6137f5b94a0b1c0`.
Backfilled records are explicitly retroactive. The mandatory host-side gate is
`scripts/verify_evidence_pre_push.sh`. ISO images, network timestamp requests,
signing-key operations, GitHub push, and physical burning remain distinct
permissions.

G14c adds production `sqlite-image+packed-assets.v1` creation and read-only
admission under `internal/snapshotimage/` and
`internal/store/sqlite_snapshot.go`. `notriosctl snapshot create` uses SQLite
Online Backup, securely clears the reviewed local/transient tables in the copy,
packs database-declared local blobs/source bundles into deterministic stored
USTAR files bounded at 256 MiB payload or 65,536 entries, resumes only verified
pack boundaries, restarts the image, and publishes the manifest last.
`notriosctl snapshot verify` checks exact schema/application capability,
hashes/lengths, SQLite integrity, identity, vector/floors, cleared state, safe
paths, deterministic headers, object hashes, and external completeness. It
returns install-ready staging only. G14d adds `snapshot restore --intent
replace|adopt`: a verified emergency physical snapshot, adjacent durable
roll-forward plan/startup blocker, staged activation with a fresh replica ID
and catch-up floors, external-index rebuild queue, and post-snapshot replay.
REST now produces and verifies the same physical snapshot through deterministic
sequential USTAR plus existing NBK1 frames; range resume beyond 3 GiB and
identical resumable directory bytes are tested. Generated 100k restore evidence
completed in 13.080 seconds at 24,788,992 bytes peak RSS and queued all 100,000
documents. No schema, compressor, dependency, REST/MCP path surface, or
archive-v2 behavior changed. G14c is archived as
`plans/v0.7/021-scalable-native-snapshot-representation.md`; G14d is archived as
`plans/v0.7/022-scalable-restore-catchup.md`; G14e is archived as
`plans/v0.7/023-full-scale-archive-catchup-acceptance.md`. Neither exFAT, a
cloud-provider rerun, an Android emulator, nor a physical device is claimed by
G14e.

G14e's resumable production harness reused unchanged import results but
re-fingerprinted the equivalent 382,206-document Joplin/Obsidian views and the
attachment workload. Nineteen privacy-sanitized phases pass: current/previous
packed archive-v2 verify/restore, physical first/verify/unchanged/restore for
both workloads, full REST/directory catch-up with emergency replacement and
post-vector replay, and frozen Restic/Borg integrity checks. The final catch-up
completed in 2,948.669 seconds at 210,010,112 bytes peak RSS with exact
canonical equality. Full scale exposed and fixed four G14d contract defects:
the ordinary 30-second timeout on synchronous snapshot creation, an 8 MiB
generic response ceiling truncating 16 MiB ranges, unconditional whole-library
reconciliation for a body edit, and quadratic repeated-prefix validation in
bounded directory publication. Tests cover each boundary; no format, schema,
compressor, third-party dependency, REST/MCP path surface, or automatic restore
was added. G15-G17b are complete. G17b sealed 81 curated artifacts, produced
the verified release ZIP and immutable reserve described above, and committed
the finite outer-catalog closure. The generated DigiCert pilot and all
production tokens passed policy/nonce/imprint/EKU/time/explicit-chain checks;
Sectigo was not used. G18f is complete; G18g is next but unapproved. Every
remaining item requires separate item-by-item user approval, and every future
push must first pass the G17b gate.

G14b added only investigation/prototype code and aggregate evidence. Its 57
validated full-corpus phase rows cover equivalent 382,206-document Joplin and
Obsidian views, an attachment-bearing workload, both native candidates, current
catch-up boundaries, stopped/online SQLite variants, Restic/Borg canonical and
1,237,553-file raw references, corruption refusal, unchanged snapshots, and a
distinct Google Drive copy. The image path was 1,884.2 seconds locally versus
4,384.0 for packed semantic reconstruction (2.33x faster), or 2,245.1 versus
4,457.6 seconds including provider evidence. The semantic artifact was 78.3%
smaller and remains first-class. Loose layouts failed file shape; raw
repository paths repeatedly failed memory. Import time/RSS remains separate
performance debt; G14e fixed the 3.22 GiB post-snapshot replay by scoping
reconciliation to admitted record families. G14b itself changed no production
format, schema, dependency, default, encryption, or catch-up behavior changed.

G14a adds only evidence/prototype code. Its 11 adapters map loose/packed
archive-v2 and catch-up, stopped/online/bundled SQLite-image candidates, and
restic/borg raw/canonical references onto nine common stages. Eighteen generated
phase rows validate immutable resume, source-read-only behavior, privacy,
arithmetic, exact wrapper hashes, semantic restore, and real incremental replay.
At 100k, loose export made 100,093 files; ZIP added 11.15% while NBK1 added
6,004 bytes; open and restore stayed under 92 MiB, but incremental replay
reached 797,937,664 bytes peak RSS. Those are baseline failures for G14b, not
production changes made by G14a.

G0 added no production sync code or dependency. It freezes the threat model,
normative glossary, thirty misuse/control traces, and upstream license/platform
matrix. Later crypto design must advance the encryption epoch when a compromised
replica is revoked, sign domain-separated canonical outer artifact bytes while
binding the visible header as AEAD associated data, and avoid exposing plaintext
content hashes as carrier routing names. Current REST still has no general
authentication and remains local/loopback-only.

G1 likewise added no production sync code, schema, or dependency. Its
aggregate-only scan covers 2,063,061 bodies without committing private paths or
content, and its deterministic workload selected complete UTF-8 revision
objects, optional beneficial named-parent line deltas, and line-first merge
with bounded Unicode-aware word-token refinement. Same-token and delete/edit
overlap becomes a durable typed conflict. The earlier
`github.com/epiclabs-io/diff3` recommendation is now superseded: it does not
provide binary delta encoding, and G7 will own a bounded pure-Go line/word merge
implementation.

G1a added an investigation-only pure-Go Subversion-style matcher, a bounded
constrained RFC 3284 VCDIFF codec, and a minimal private comparison container
under `performance/`; it added no production sync code or dependency. All 21
text/binary fixtures reconstructed exactly and deterministically. VCDIFF was
beneficial in 19 cases and smaller than G1's line JSON in all 14 comparisons;
empty and unrelated bytes retain complete-object fallback. Pinned xdelta3 and
open-vcdiff oracles decoded every representative Go stream exactly. The strict
Go decoder is not a general VCDIFF decoder, and the private container and
Subversion svndiff are not selected. Production use requires G2 bounds, a real
immutable named parent, exact outer hashes, and separate G7/G8 approval.

G2 also added no production sync code, schema, cryptography, transport, or
dependency. Its aggregate-only 100/10k/100k workload selected compact canonical
NCB1 operation records plus a canonical-JSON outer manifest candidate and
deterministic gzip: at 10,000 operations NCB1 was 51.9% smaller raw, 16.3%
smaller compressed, and materially cheaper to decode/allocate than the JSONL
prototype. Envelopes close at 10,000 operations, 16 MiB canonical bytes, or 4
MiB compressed bytes. Per-peer pending admission stops at 10,000 operations/64
MiB disk-backed bytes. Resources stay whole below 1 MiB and use 1 MiB fixed
chunks above it; sync packs provisionally target 64 MiB/4,096 objects/4 MiB
trailers. G9 must still promote or replace the codec, pin goldens, and implement
crypto/admission. FastCDC remains deferred. Emulator and physical-device
checklists prevent these desktop-proxy numbers from becoming a mobile claim.

G3 added production local profile/config isolation but no replication schema or
transport. The version-2 stable-link registry now supports generated runtime
profiles with a random local profile ID, one bound database/replica identity,
one owner-only config, isolated absolute runtime paths, a distinct loopback
port/public URL, and `sync.target: none` by default. `notriosctl profile
create|show|list|validate|start` is live; start launches only `notriosd -config
<path>` and is not a supervisor. Startup independently revalidates the binding.
Raw filesystem copies are refused until explicit adopt/fork, while two valid
replicas of one database remain explicit stable-link ambiguity. Status/UI name
the active profile. The real CLI fixture runs two daemons simultaneously.

G4 adds schema-v19 local replication durability but no transport, admission,
merge, cryptography, REST/MCP surface, or UI. Explicit enrollment records a
sequence-zero full-snapshot boundary for the current replica. Canonical-table
triggers feed one transient seam that allocates an immutable local operation
and advances the local contiguous vector inside the caller's transaction;
rollbacks and interrupted uncommitted transactions retain neither side.
`target: none` before enrollment stays journal-free, while a non-none target
establishes the boundary at startup. Identity rotation retires the allocator
and requires re-enrollment. The 100k import A/B produced exactly 300,000
operations with 23.9% elapsed and 92.8% database-byte overhead.

G5 advances the schema to v20 and adds transport-neutral admission, but still
no carrier, cryptography, REST/MCP/UI surface, background sync, or canonical
record merge/application. `internal/syncstate` fixes protocol 1.0 with schema
19-20 compatibility and three required capabilities, compares bounded vectors,
and plans deterministic missing ranges. Already configured local fixture peers
can submit strict normalized operations; gaps and missing dependencies remain
disk-backed under the G2 quotas, exact replay is inert, conflicting replay and
unknown records refuse, and one transaction moves all newly contiguous work
into the immutable operation set while updating gaps/vector/ack. A handshake
never auto-enrolls a peer. Three real local replicas plus a 100-seed model
converge after shuffle, duplicate, and drop-then-deliver schedules; restart,
injected rollback, clock/sequence skew, and sequence exhaustion retain the
correct boundary.

G6 advances schema v21 and atomically applies G6-owned canonical metadata after
G5 admission. Bounded HLCs order sparse field registers and LWW document-tag
elements by wall/logical/replica/sequence; per-replica HLC regression refuses
the whole admission. The fold starts at the explicit sequence-zero baseline,
keeps trash/restore distinct from permanent death certificates, and repairs
notebook cycles/orphans, document homes, and case-insensitive name collisions
deterministically with visible current repair rows. Sync-enabled local purge is
gated until G9 supplies signing; the internal G6 fixture seam only validates
mandatory structural signer/signature fields. G6 does not merge bodies, move
resource bytes, collect retained payloads, add a carrier, expose sync over
REST/MCP/UI, or authenticate peers.

G7 advances schema v22 and converges note bodies. A revision is an immutable
object naming its parents, its exact content hash, and its byte length; the
capture trigger refuses an enrolled revision without one, and the upgrade
backfills every existing revision plus a synthesized linear parent chain,
breaking `created_at` ties by insertion order rather than by random id.
`internal/syncdelta` is the reviewed promotion of the G1a VCDIFF prototype with
production bounds and a benefit gate; the unselected `NXD1` container and the
stream wrappers were not promoted. `internal/syncbody` implements G1's bounded
line-first merge with **single-line** word refinement — two randomized cases
proved a wider region invents lines — plus the revision DAG and the derived
merge and conflict identities. Admission verifies a reconstructed body against
its exact hash before any canonical write; a missing base or corrupt patch
becomes `missing_base` or the terminal `unverified` in
`sync_revision_pending_bodies`, never a best-effort patch. Overlapping edits
become a durable typed conflict on the same document with its two revisions
stored sorted, so both replicas derive one identity. `current_revision_id` is
derived from the revision graph, not last-writer-wins. Real two-replica evidence
transferred 55.37%, 27.92%, 16.49%, and 12.06% of the complete-body
counterfactual at G1's four offline intervals with every document converging.
G7 does not move resource bytes, add a carrier or wire codec, sign anything, or
expose revisions, deltas, or conflicts over REST/MCP/UI.

G8 advances schema v23 and converges attachments before their bytes. A blob row
may now exist without a file — `blobs.availability` is `local` or `unavailable`,
and a trigger refuses in both directions any row whose availability contradicts
whether it has a storage path — so a note can reference an attachment this
replica has not downloaded, with no placeholder bytes anywhere.
`internal/syncassets` holds G2's whole-below-1-MiB and 1-MiB-chunk plan, the
16,384-chunk and 16 GiB ceilings, manifests with per-chunk hashes and a
content-addressed digest, and the eager/pinned/lazy policy whose threshold is
deliberately the same mebibyte that decides chunking. Bytes are fetched through
a transport-neutral `ObjectProvider` that G11 and G14 will implement; chunks are
staged outside the content-addressed tree, transfers resume from verified
segments, and nothing is installed until the manifest digest, each chunk hash,
the whole-object hash, the length, and the sniffed content type all agree.
**Two behaviors changed elsewhere on purpose:** an archive-v2 export now refuses,
naming the object, rather than omitting unmaterialized bytes, and garbage
collection removes an unmaterialized blob's transfer state and staged chunks
with it. Resource deltas were considered and not implemented — G1a's benefit
case needs a named immutable parent, which resources do not have.

G9 adds `internal/syncwire` and **no schema change and no dependency**. It is
the canonical NCB1 operation block, the NEV1 envelope, deterministic gzip, and
the NAR1 artifact: AES-256-GCM under a key derived per artifact by HKDF-SHA256
from a fresh 32-byte salt, signed with Ed25519 over domain-separated canonical
outer bytes. Encrypt-then-sign lets a receiver reject a forgery without
decrypting, and the canonical header is both the derivation salt and the AEAD
associated data. The visible header carries only G0's routing tuple, with
routing names as keyed HMAC blinds rather than plaintext content hashes.
Advancing an encryption epoch and retiring one are separate acts, so revocation
does not cost a library its own history. Every primitive is Go standard library.
**Two things a later agent should know:** G2's fixed sixteen-byte identifier
assumption did not survive production identifiers and was replaced with length
prefixes, so the promoted codec is not byte-identical to the prototype that
justified it; and `SignDeathCertificate`/`VerifyDeathCertificate` supply the
signing G6 recorded as owed, but the enrolled-purge path is deliberately **not**
switched over — that belongs with G17's retention horizon. The store still
journals and admits JSON operations locally; the canonical codec is a wire
format, not the journal's storage.

G10 advances schema v24 with the durable catch-up state machine. Requests and
responses are signed; only an active enrolled peer **explicitly permitted** as a
snapshot source may answer; competing offers are chosen among, never merged; and
the state machine is an explicit transition table in which a restore in progress
cannot re-fetch underneath itself, be cancelled, or be expired by a clock.
Cutover requires an explicit restore intent and writes no peer acknowledgement.
**The thing a later agent most needs to know:** a replica built from a snapshot
has no predecessor operation rows, which broke G5 admission outright until
`sync_catchup_floors` was added. A floor is the only thing permitted to stand in
for a missing predecessor, and a replica without one still leaves such
operations pending. Password wrapping is Argon2id, which promotes
`golang.org/x/crypto` from indirect to direct at the same version.

G11 adds `internal/synccarrier`, `internal/synckeys`, and `notriosctl sync`, and
**no schema change**. The shared folder is a disposable postbox: every path
segment below `notrios-sync/v1/` is a keyed blind, every artifact is a G9
sealed artifact, and every writable path lives inside the writing replica's own
namespace. **Three things a later agent should know.** First, the illustrative
layout in `SYNCHRONIZATION.md` was wrong and is corrected there — object paths
published plaintext content hashes, envelope names published sequence ranges,
and a separate acknowledgement class duplicated what a contiguous vector already
says. Second, artifacts are named by what they logically are rather than by
their sealed bytes, because every seal draws a fresh salt; a publisher
republishes when its own copy is *unreadable*, not merely absent, which is what
repairs a torn artifact and what keeps a quiet carrier from growing. Third, a
round starts from what the journal remembers each peer acknowledged rather than
from what the folder says, so an empty or deleted carrier is not a standoff and
removable media converges in two trips. Measured: the carrier layer is about 1%
of an exchange — SQLite admission is the rest. `internal/synckeys` is the warned
`0600` development secret provider; v0.8 still owns the platform store, and G13
still owns real pairing.

G12 changed **no production code**: it is the evidence run that puts G11's
carrier on a real provider. Two measurements from it constrain later work and
now live in `SYNCHRONIZATION.md`: through a Google Drive `rclone mount`, another
device's change took **45-57 seconds** to become visible, and **resolving a
known name is no fresher than listing the directory**. So publication order —
envelopes first, advertisement last — is a latency optimization on such a
carrier and *not* a correctness mechanism; a phase makes the advertisement
visible without its envelopes and asserts the reader claims no progress it did
not make. G15 should not schedule polls faster than a provider announces
changes. `docs/operations.md` now carries the operator section, the safe
commands, and the never-run rclone verbs, whose refusal the harness enforces in
code rather than in a comment.

G13 advances the schema to **v25** and adds the first authenticated surface this
project has ever had. A peer principal is one enrolled replica of one database,
proved by an Ed25519 signature over the method, path, database id, replica id,
timestamp, nonce, and body hash — never a bearer token. It authorizes
`/api/v1/sync/...` for that database **and nothing else**; a test compares an
ordinary note route's answer with and without a peer credential and requires
them identical. **Three things a later agent needs to know.** First, peer public
keys are database state now (`sync_peer_keys`), not key-file entries, so
enrolment and revocation are transactional and audited, and the carrier's
verifier became `syncwire.MultiVerifier{own key, database peers}`. Second,
G11's clear-text development bundle is **gone**: pairing is a short-lived,
single-use code under which the group key travels sealed, and `sync
bundle|pair` were replaced by `sync invite|join|accept|enroll`. Third, the
transport policy is a **startup refusal** — with the surface enabled, a
non-loopback listener without TLS makes `notriosd` exit and name the setting.
No data plane landed; G14 owns it.

G14 adds the REST data plane and **no schema change**. `internal/syncrest`
implements G11's `Carrier` over G13's signing client, so REST and a shared
folder are one protocol with two couriers — transcript parity by construction,
asserted by comparing what each carrier holds. Measured, REST costs +15.4% at
100 notes and +0.07% at 500 against the folder, so keeping the merge off the
server costs nothing. `internal/syncbackup` packs, seals in fixed authenticated
frames, and extracts safely; a downloaded snapshot passes four gates in order —
declared hash, frames, sequential USTAR container, **then the strict physical
snapshot verifier** — and backups
are addressed by opaque id, produced only for an explicitly permitted replica,
and fetched only by the one that asked. **The thing a later agent most needs to
know:** G13's per-address failure budget was being spent on every request rather
than on refusals, which one request per authentication hid and a data-plane
round exposed immediately as a `429` against a legitimate peer. It is now
checked before work and spent only on refusal, pairing excepted, and the
per-peer request budget rose from 120 to 600 a minute.

The 2026-08-15 scalability amendment reopened only the **physical full-snapshot
representation**, not G14's security or transport contract. G14 exported loose
archive-v2 objects, then stored every one as a ZIP entry before frame sealing;
at 100 and 500 notes that added about 25%, almost entirely ZIP entry metadata.
G14a-G14e completed that review and froze the compatible physical default.
G15 may now start only after explicit user approval; it must preserve this
format split and must not turn synchronous backup creation into an unbounded
ordinary API operation.

That review also added a second portability route. v0.8 now investigates and
builds a framework-neutral Go application facade and versioned no-GUI C ABI,
with Android-emulator-only pre-1.0 evidence; v1.0 packages the supported ABI
matrix; and a post-1.0 Flutter client owns physical mobile and native-desktop
delivery. Dart FFI is not the Flutter Web bridge. `FLUTTER_GO_CLIENT.md` records
the API/lifecycle/ownership/stream contract and source checks. The current GUI's
Mermaid support is **disabled**, not merely untested (`noMermaid: true`), and a
v0.8 offline/security-tested enablement slice owns it.

Latest completed feature validation is G14 (2026-08-14): two replicas
converging over REST with an asserted identical transcript, `206`/`416` range
behavior, an interrupted snapshot download resumed, verified, restored under an
explicit intent and then continuing incrementally, a tampered snapshot refused
at the transport's own hash, a backup refused to a replica that did not ask for
it, and the syncbackup frame suite. The preceding validation is G13
(2026-08-13): the seventeen-case
authentication matrix, the syncauth binding/replay/skew/limiter suite, the
store's enrolment and single-use-invitation fixtures, ten transport-policy
rows, and a cross-process pairing over a live `notriosd`. The preceding
validation is G12 (2026-08-13), whose eight conformance phases ran the shipped
round against a mounted Google Drive folder and a drive passed between peers,
and before that G11
(2026-08-13), which added the
synccarrier suite — two- and three-replica convergence through a real folder,
carrier deletion and republication, truncation, unenrolled signers, unpaired
replicas, foreign namespaces, provider sidecars and wrong-case names, idempotent
publication, cleanup refusing another namespace, correctness with cleanup off,
an unavailable mount, the no-rename fallback, vanished and oversized entries,
stable listing order, concurrent writers, removable media, attachment bytes on
request — plus a three-test multi-process `notriosctl sync` fixture and a
carrier-wide assertion that no title, body, replica id, or database id appears
in any path or byte of the folder. The preceding feature validation is G10
(2026-08-13). It includes G5's
admission suite plus exhaustive 40,320-order and 250-seed convergence models,
opposite-order real SQLite replicas, sparse-register, membership, lifecycle,
tree-repair, clock-regression, restart/upgrade, and purge-gate checks, followed
by the audit-first full repository, frontend, docs, smoke, and release checks
recorded in its archive. Regular
validation begins with audit/fix/reinstall/re-audit, while CI and release
packaging enforce a non-mutating audit gate. G7 added its own suite on top:
the syncdelta round-trip/hostile/limit/fuzz set, the syncbody merge and
merge-base fixtures with 4,000 committed randomized merges, and real two-replica
store fixtures covering clean merges, durable conflicts, four broken-delta
refusals, tampered operations, six delivery orders, delete/edit, restore/edit,
and the v22 upgrade backfill, plus G8's syncassets chunk/manifest/policy suite
and real two-replica attachment fixtures covering hostile sources, resume,
dedupe, restart, and the v23 backfill, plus G9's independently generated goldens, RFC/NIST
known-answer vectors, exhaustive tamper cases, and two fuzz targets, plus G10's
permission, offer-selection, state-machine, floor, and password fixtures. Start
G11 only after explicit user approval, then stop after its validation, commit,
verified ZIP, and handoff before G12.

Two things a reader continuing this project should know about v0.6:

- **The v0.6 half-shipped job bullet is now resolved for sync.** Import/export/
  snapshot jobs remain watching-only because they name local paths. G15 adds a
  distinct closed, path-free start/control surface for incremental/resource
  sync only; it does not make arbitrary jobs remotely startable.
- **F7's reconciliation found three defects**, all fixed in it: the batch route
  applied `trash` and tag operations to notes in read-only notebooks that the
  single-note routes refuse, single-note tagging had no read-only guard at all,
  and six REST surfaces had neither an MCP tool nor a recorded reason.

Milestone detail follows. H1–H11 are archived under `plans/v0.3/`. The v0.4
slices are archived under `plans/v0.4/`:

- J1–J3: canonical Joplin RAW parsing, bounded relationship planning, and
  million-item transactional import throughput;
- Q1: bounded boolean/category search;
- P1: the shared selection/privacy planner;
- P2–P3b: the archive-v2 format and identity contract, streaming export, the
  large-library container revision, and the optional packed object layout;
- P4: verify and restore under mandatory intent;
- P5: stable external links and local resolution;
- P7: publication profiles and the privacy-reviewed handoff;
- P8: documentation and release wrap-up (version 0.4.0, the v0.4.0 release
  checklist, a verified Help reseed, and the v0.5 draft in `PLAN.md`).

P6, the `movenotes-v3` compatibility bridge, is deferred to v0.7 G19 and gated
on G9, which stabilizes the sync-era container capabilities it would pin.

Archive-v2 supports two object layouts. Loose `fanout` is the default and
deduplicates and resumes through the object tree. Opt-in `--pack` collapses a
382,206-note archive from 382,447 files to 46 at ~11% more disk and 1.29×
faster; the file-count collapse, not local speed, is what v0.7's REST and
folder/rclone transports need. That earlier conclusion is now a hypothesis to
retest end to end in G14b rather than the final default.

## First files to read

1. `AGENTS.md`
2. `README.md`
3. `PLAN.md`
4. `ROADMAP.md`
5. `CODING_CLIENT_HANDOFF.md`
6. `SYSTEM_ARCHITECTURE.md`
7. `API_SPEC.md`
8. `DATABASE_SCHEMA.md`
9. `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`,
   `RECOLL_INTEGRATION.md`, `SYNCHRONIZATION.md`, `DOCS_SITE.md`
10. `TESTING_POLICY.md`, `ENVIRONMENT_SETUP.md`, `CONTEXT_MAP.md`
11. `agent/PLAN_STATUS.md`, `agent/ATTEMPT_LOG.jsonl`, `agent/MODEL_LOG.jsonl`
12. Release/history context when needed: `plans/mvp/MVP_RELEASE_REPORT.md`, `RELEASE_CHECKLIST.md`, `SECURITY_REVIEW.md`, `PACKAGING.md`

## Current state

The completed v0.1 MVP supports:

- `notriosd` local HTTP service;
- SQLite database creation, migration bootstrap, and default managed collection bootstrap;
- document create/read/update/patch/soft-delete;
- revision list/read/restore;
- SQLite FTS5 search over current non-deleted notes;
- content-addressed resource upload, metadata, content streaming, attachment, detachment, and safe delete;
- Markdown link/backlink parsing and graph slices;
- React/Vite UI using `md-editor-rt` with document/resource link routing;
- dependency-free MCP endpoint at `/mcp` (read-only default; editor writes);
- `notriosctl import joplin-raw` and `notriosctl import obsidian`;
- generated-dataset smoke/performance tests;
- release packaging and ZIP verification scripts.

The completed v0.2 redesign added notebooks/tags/search notebooks, source
provenance and threads, query language, optional Recoll, five importers, native
archive v1 interchange, Wails v2 GUI, and docs site. v0.3 added the
remote-media policy/scan/quarantine/localization surfaces, exact duplicate and
unreferenced resource reports, per-notebook resource usage, and an optional
review-only perceptual hook that is inert by default. H6 added schema-v8
resource retention state, configurable local retention, dry-run-first CLI
garbage collection, a read-only REST report, explicit confirmation for
permanent REST deletion, and a future sync-aware retention gate. Archive v1 is
not a full backup; P2 defines and verifies the v0.4 native archive-v2
full-snapshot/container layer reused by v0.7 sync, P3/P3a/P3b export it at real
library scale under two object layouts, and P4 verifies and restores it under
explicit replace/adopt/merge/fork intent. H7 added schema-v9 keyset
indexes and query-bound cursors for chronological/relevance, route-bound
notebook/Trash paging, live GET search, and bounded immutable snapshots for
optional FTS5/Recoll merging. Reproducible 10k/100k/500k evidence lives under
`performance/v0.3-h7/`. H8 added schema-v10 importer checkpoints and
fingerprints, exact optional source bundles, Joplin nested notebooks and stable
real tags, bounded batch lookups, dry-run/config parity, stable resource
refresh, interruption/resume, and generated 100/10k/100k evidence under
`performance/v0.3-h8/`. H9 applies the same generic state model to Obsidian:
nested vault notebooks and collision renames, exact Markdown/frontmatter and
non-Markdown source capture, alias/relative/embed/heading/block
canonicalization, stable resource refresh, resume/dry-run parity, and generated
100/10k/100k/500k evidence under `performance/v0.3-h9/`. H10 added schema-v11
durable projection retry/backoff, bounded drains, exact missing/stale/orphan
reconciliation, hardened cancellable Recoll processes/output, stable
deduplicated per-hit engine attribution, status/UI observability, and real
100k native Recoll evidence under `performance/v0.3-h10/`. H11 added the
cross-cutting maintenance guide (also reseeded into Help), reconciled living
specs and feature status, bumped product metadata to v0.3.0, completed the
release-candidate gates, and drafted the v0.4 plan. The 2026-08-02 follow-up
selected an external `movenotes-v3` archive bridge instead of duplicating
Obsidian/Quartz/Hugo publishing, implemented bounded boolean/category search,
and completed the J1–J3 Joplin prerequisite slices plus Q1. See
`agent/PLAN_STATUS.md`.

## Validation commands

Run these before committing any task:

```bash
go vet ./... && go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm audit && npm audit fix
cd web && npm ci && npm audit && npm run typecheck && npm run build && npm test -- --run
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/run_joplin_import_profile.sh 100 /tmp/notrios-joplin.json
bash scripts/run_real_joplin_profile.sh <label> <raw-export-dir> /tmp/notrios-joplin-real.json
bash scripts/run_obsidian_import_profile.sh 100 /tmp/notrios-obsidian.json
bash scripts/run_recoll_hardening_profile.sh 100 /tmp/notrios-recoll.json
```

GUI-affecting tasks also build with `make gui`; layout changes additionally run `scripts/verify_layout_resize.py` under Xvfb/Openbox (see `TESTING_POLICY.md`).

## Git workflow and state (reviewed 2026-07-26)

`main` takes reviewed merges; active work happens on `develop`. Commit each completed working-state slice; pushing to GitHub is the **user's step**.

Current handoff facts (verify again before acting):

- `origin` is configured (`https://github.com/renesugar/notrios.git`).
- `develop` review base was `26b0925`; it has no configured upstream.
- local `main` was `265ef4e` tracking `origin/main`.
- The v0.3 commits are local only. The user explicitly prohibited a GitHub
  push for this session.

Commit-message convention: each agent ends commit messages with its own `Co-Authored-By:` trailer, and appends its model to `agent/MODEL_LOG.jsonl` at session start (see `AGENTS.md`).

## Important constraints to preserve

- SQLite is the canonical managed-note database.
- Recoll is a derived, optional, external search sidecar — never canonical storage, never linked/vendored (GPL; see `RECOLL_INTEGRATION.md`).
- Project code must remain compatible with an MIT or Apache-2.0 license.
- MCP must not expose raw SQL or arbitrary filesystem operations.
- Imported Markdown and downloaded resources are untrusted; preview HTML must be sanitized.
- Remote media localization must go through media policy and quarantine checks.
- Exact SHA-256 is the only deduplication identity. Perceptual hashes are
  review suggestions only and no algorithm ships by default.
- Resource garbage collection must remain dry-run first, transactionally
  recheck references on apply, and pass future synchronization acknowledgement
  policy through `store.RetentionGate`.
- Every task must leave the repo in a working state; archive completed plans under `plans/`.
- Unbounded paging uses keysets/snapshots, not hidden offsets.
- Sync must follow `SYNCHRONIZATION.md` and the approved item in `PLAN.md`:
  canonical local stores, immutable operations/objects, contiguous state
  vectors, revision-aware merge, lazy resources, ephemeral-directory and REST
  adapters, secure snapshot catch-up, and acknowledgement-gated retention. The
  directory is disposable; rclone is a test carrier only, and `rclone sync` is
  not the merge algorithm.

## Local environment notes

Facts about the development machine that no other document records:

- `./data/` in the repo root holds a throwaway development database (`notes.sqlite` plus `assets/`, `quarantine/`, `projections/`, `search-index/`) created by ad-hoc service and GUI smoke runs. It is gitignored and safe to delete; the service recreates it on startup.
- `web/dist/` is gitignored; run `cd web && npm ci && npm run build` after a fresh clone (CI and `scripts/package_release.sh` build it too).
- `scripts/verify_layout_resize.py` needs Python `playwright` plus `xdotool`, `Xvfb`, and `openbox` (all installed system-wide here, but **no Python venv is committed** — create one with `python3 -m venv … && pip install playwright`; it can drive the system `google-chrome`, so no browser download is required).
- `npm test` under Node 22 prints a harmless `ExperimentalWarning: localStorage` — the real polyfill lives in `web/src/test/setup.ts` (explained in its comments).
- MCP write-tool tests may still exercise the deprecated `DefaultProfile`
  field as a compatibility case; new tests and configuration use
  `DefaultScope`/`mcp.default_scope`.
- The GUI before/after screenshots from the v0.2 conformance pass live in `/home/renes/prompts/` (outside the repo, intentionally uncommitted — transient browser-automation output is never committed).
- All verification servers, Xvfb displays, and browser sessions from prior agent sessions are stopped; session scratchpads lived under `/tmp` and are disposable.

## Environment limitations inherited from the scaffold

The scaffold was created in a restricted container. Still-open consequences:

1. The SQLite store uses a small local cgo adapter over the vendored SQLite amalgamation (v0.8 H1 slice C; previously system `libsqlite3`) on
   Linux. v0.8 H0 must compare a pinned C amalgamation with pinned modernc/libc
   in the Go Android shared core before implementation. The modernc Android
   build is not runtime/support evidence; Jetpack's Kotlin driver still requires
   a database-ownership redesign.
2. The MCP adapter is dependency-free; the official MCP Go SDK can replace it later without changing tool semantics.
3. J1 matches canonical Joplin first-line titles and CR/LF-only metadata
   parsing, including OCR controls. J2 validates both supplied real exports and
   makes relationship planning proportional to parsed links. J3 adds bounded
   atomic canonical/checkpoint batches, an indexed temporary manifest, final
   links, and private-safe complete recipe-corpus evidence under
   `performance/v0.4-j3/`. Never commit private datasets or content-bearing
   evidence.
4. Q1 uses one bounded AST for uppercase `OR`, implicit `AND`, prefix
   negation, grouping, phrases, fields, and `category:`/`notebook:` aliases.
   SQLite is exact for every expression; supported shapes compile to Recoll
   with live parity tests, and unsupported sidecar shapes fall back explicitly
   to canonical SQLite rather than being approximated. Generated evidence is
   under `performance/v0.4-q1/`.
5. P1 provides one read-only Store/REST/MCP planner for recursive
   notebook/tag/query/explicit-ID selection. Target policies classify reachable
   resources and internal/private/broken links, hash source-bundle keys, strip
   paths/private metadata from API output, cap visible details, and bind the
   complete manifest to SHA-256. Generated 100k evidence is under
   `performance/v0.4-p1/`.
6. P2 adds schema-v12 stable logical database and per-writable-copy replica
   identities plus a separate read-only archive-v2 verifier. The manifest-last
   format uses strict typed JSONL records and immutable SHA-256 body/resource/
   source-bundle objects; schema/capability/MIME/size/count/path/depth and
   cross-reference checks complete before restore writes. Synthetic
   golden/adversarial fixtures live under `internal/archivev2/testdata/`.
7. P3/P3a/P3b make the container hold a real library: the object inventory
   lives in checksummed index chunks under an `ab/cd` fanout, the writer and
   verifier stream through external-sorted spools, and an optional `--pack`
   layout collapses file count behind the `objects.pack.v1` capability.
8. P4 adds `verify archive-v2` and `restore archive-v2 --intent
   replace|adopt|merge|fork`. Restore completes verification before its first
   canonical write, reads both layouts, re-hashes bytes at use, re-sniffs blob
   MIME, and records a schema-v13 `restore_state` marker so an interrupted
   restore cannot pass as a complete library. Resource and source-bundle
   coverage comes from the attachment-bearing Joplin corpus
   (`performance/v0.4-p4/`); neither recipe corpus carries attachments.
9. P5 adds the external `notrios://databases/{id}/documents/{id}` link, the
   strict `internal/stablelink` parser, the explicit `internal/profiles`
   registry (`~/.config/notrios/profiles.json`, override with `--registry` or
   `NOTRIOS_PROFILE_REGISTRY`), `POST /api/v1/links/resolve`, and the
   `notriosctl link|open|profile|register-url-handler` commands. Resolution is
   local routing only: it never contacts a peer, never scans the filesystem,
   and refuses rather than choosing when several profiles hold clones of one
   database.
10. P7 adds `notriosctl publish profile|plan|run`. A publication is a
    projection, not an archive of canonical state: current revisions only, no
    Trash/provenance/source bundles/saved searches, stripped revision metadata,
    links to withheld or unresolved targets rewritten, and their link records
    dropped. Publishing requires the digest of a reviewed plan and re-checks it
    before writing. Content-rewriting link actions remain refused for
    `full_archive`.
11. E4 replaced the MVP graph slice with bounded traversal: `POST /api/v1/graph`
    honours `depth` (it was declared and never read), `POST /api/v1/graph/path`
    finds a shortest path from both ends, and `GET /api/v1/graph/report` lists
    orphans, isolates, and in-degree hubs. A bound wider than a ceiling is
    refused rather than clamped; a traversal stopped by one reports
    `truncated_by` and `completed_depth`; and `no_path` is kept distinct from
    `depth_exhausted` and `budget_exhausted`, because only the first is a
    statement about the library.
12. E5 adds `GET /api/v1/links/suggest` (bounded title autocomplete returning
    IDs and titles only) and `POST /api/v1/links/check` (read-only resolution of
    an unsaved buffer, parsed by the canonical extractor so markers match what a
    save records). Schema v16 replaced `lower(title) = lower(?)` with a NOCASE
    index, which had made every title-resolved link a full scan of the document
    table on every save and every lint pass.
13. E6 weighed migrating to CodeMirror 6 and **declined**: `md-editor-rt` 6.5.3
    *is* CodeMirror 6 and exposes it (`completions`, `codeMirrorExtensions`,
    `getEditorView`, `domEventHandlers`), so the capabilities E5 wrongly recorded
    as unavailable were available all along. In-editor `[[` autocomplete, broken-
    link underlines, and Ctrl-click were implemented through those hooks for
    1.3 kB gzipped and no measurable typing cost. See `PROJECT_DECISIONS.md` 20.
14. E6a made the UI offline-capable. It had been fetching KaTeX, highlight.js,
    echarts, cropperjs, and prettier from `unpkg.com` at runtime — 13 requests,
    623 kB, on every launch — and math silently rendered as raw LaTeX without a
    network. Those are bundled or disabled now, `handleWebApp` serves a
    Content-Security-Policy, and `scripts/run_offline_assets_check.sh` fails if
    any of it returns. Cost: 151 kB gzipped and ~400 ms of first contentful
    paint, both measured.
15. E6b converts a pasted HTML table into a Markdown pipe table, so blocks, link
    extraction, and portable export can see into it. It refuses far more than it
    converts — merged cells, ragged rows, nested blocks, multi-line cells, a
    paste that merely contains a table — and every refusal falls through to the
    ordinary paste, so nothing pasted can be lost. Parsing is inert `DOMParser`;
    no HTML is re-emitted.
16. E7 renders a fenced ```note-query block through
    `POST /api/v1/note-queries/run`, which parses the block server-side with the
    same Q1 parser every search surface uses. A malformed block is a 200 with
    `error` so the note still renders. `SearchRequest` gained an explicit `Sort`
    because the order used to be implied by the query's shape. A publication
    carries the block's text, never a materialized result — asserted by test.
17. E8 puts trash-first deletion in the GUI — Move to Trash, Restore, Delete
    forever — and confirms a notebook deletion with the service's own
    `GET /api/v1/notebooks/{id}/deletion-preview`, including the re-homing rule
    that gives a later restore somewhere to land. `store.RenameTag`,
    `POST /api/v1/tags/rename`, and `notriosctl tags rename` add hierarchical
    tag rename whose **dry run is a rolled-back apply**: the real statements run
    inside a transaction, so a dry run and an apply cannot disagree. Dry run is
    the default on both surfaces. Bulk organizer operations remain v0.6.

(The formerly open "no browser testing" limitation is resolved: the GUI is browser-verified via Playwright, vitest/RTL covers the workspace, and `scripts/verify_layout_resize.py` covers native window resizing.)
