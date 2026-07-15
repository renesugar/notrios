# Release Packaging Skill

Use this skill when preparing or verifying a repository ZIP for handoff, Gitea/GitHub import, or release-candidate review.

## Steps

1. Run the full validation set:

   ```bash
   go test ./...
   python3 scripts/check_required_files.py
   bash scripts/validate-scaffold.sh
   cd web && npm ci && npm run typecheck && npm run build
   ```

2. Run smoke checks when the task affects runtime behavior:

   ```bash
   bash scripts/mvp_smoke.sh
   bash scripts/run_performance_smoke.sh
   ```

3. Package with the release script rather than ad hoc ZIP commands:

   ```bash
   bash scripts/package_release.sh /tmp/notes-companion-v0.1.0-mvp.zip
   ```

4. Verify the ZIP explicitly:

   ```bash
   python3 scripts/check_release_zip.py /tmp/notes-companion-v0.1.0-mvp.zip
   ```

## Required package properties

- Include `web/dist/index.html` and `web/dist/assets/*`.
- Exclude `web/node_modules/*`.
- Exclude `.git/*`.
- Exclude runtime `data/*` and SQLite files.
- Include docs, plans, prompts, skills, tests, scripts, Go source, UI source, lockfiles, and migrations.

## Common failure

A ZIP can look valid while missing `web/dist/`, making `notesd` unable to serve the built UI immediately. Always use `check_release_zip.py` before handing a ZIP to the user.
