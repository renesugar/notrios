# Release Checklist — v0.1.0 MVP

Before tagging `v0.1.0-mvp`:

- [ ] Choose and add a real project license.
- [ ] Confirm the repository remote and branch protection strategy.
- [ ] Run `go test ./...`.
- [ ] Run `bash scripts/validate-scaffold.sh`.
- [ ] Run `cd web && npm ci && npm run typecheck && npm run build`.
- [ ] Run `bash scripts/mvp_smoke.sh`.
- [ ] Run `bash scripts/run_performance_smoke.sh`.
- [ ] Run `bash scripts/package_release.sh /tmp/notrios-v0.1.0-mvp.zip`.
- [ ] Run `python3 scripts/check_release_zip.py /tmp/notrios-v0.1.0-mvp.zip`.
- [ ] Review `SECURITY_REVIEW.md`.
- [ ] Review `MVP_RELEASE_REPORT.md`.
- [ ] Confirm `PLAN.md` has moved to the next draft plan and v0.1 is archived under `plans/v0.1/`.
- [ ] Commit on a release branch and tag only after review.
