# Testing Policy

## Definition of done

A task is done only when:

- The code or document change is complete for the promised scope.
- Relevant tests pass or missing tests are explicitly documented.
- `agent/PLAN_STATUS.md` and `agent/ATTEMPT_LOG.jsonl` are updated.
- The repository remains in a working state.

## Test layers

### Unit tests

- Go service packages: business logic, parsers, media policy, cursor encoding, link parsing.
- TypeScript UI: API client, routing helpers, link/resource URI handling.

### Integration tests

- SQLite migrations.
- Document CRUD + revisions + FTS5 updates.
- Resource upload/download and reference counting.
- Importer fixtures.
- REST/MCP service-layer parity.

### UI tests

Planned tools:

- Playwright for browser UI smoke tests.
- xvfb on Linux CI where needed.

Minimum UI flows:

- Create note.
- Edit and save note.
- Preview note.
- Click internal document link.
- Upload/download resource.
- Search and open result.

Layout-resize testing rule: never verify window-resize behavior through
Playwright's emulated viewport (`set_viewport_size`) or by eyeballing
screenshots — the emulated canvas redraws cleanly while the real window
never changes, hiding integration bugs. `scripts/verify_layout_resize.py`
is the reference harness: it launches a headed Chromium with no viewport
emulation under Xvfb/Openbox, resizes the actual X11 window with
`xdotool windowsize`, and asserts the pane geometry (bounding rects fill
the window; editor and preview split equally after a resize) from the DOM.

### Performance tests

Synthetic datasets should eventually cover:

- 10k notes.
- 100k notes.
- 500k note-like short documents.
- 1M links.
- large resource directory with deduplication.

Performance thresholds are not defined yet; first milestones should record baseline timings.

### Security tests

- SSRF-blocking for remote media.
- Domain stop-list matching.
- Oversized download rejection.
- MIME sniffing mismatch.
- HTML sanitization for Markdown preview.
- MCP result-size limits.

## Current validation commands

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
```

Frontend validation after dependencies are installed:

```bash
cd web
npm install
npm run typecheck
npm run build
```


## MVP release validation

Task 10 adds release-candidate checks beyond ordinary unit tests:

```bash
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh /tmp/notrios-v0.1.0-mvp.zip
python3 scripts/check_release_zip.py /tmp/notrios-v0.1.0-mvp.zip
```

The generated-dataset smoke test exercises document creation, FTS5 search, link graph resolution, graph slices, resource creation, and resource attachment on a synthetic dataset. The benchmark is intentionally small enough to run on developer machines; it is not a replacement for future hundreds-of-thousands-of-notes performance testing.
