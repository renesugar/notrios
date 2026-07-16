# v0.2 Task R14 — GUI themes

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5).

## Changes

- `web/src/styles.css`: every UI color moved to CSS custom properties (`--bg`, `--text`, `--muted`, `--panel-bg`, `--border`, `--border-strong`, `--accent`, `--accent-soft`, `--hover`, `--chip-bg`).
- `web/src/themes.ts`: builtin Light/Dark themes; custom themes stored in localStorage; per-mode selection (`light mode uses X`, `dark mode uses Y`); `applyTheme` writes tokens onto `<html>` and exposes the theme's light/dark base for `md-editor-rt`.
- Header controls: 🌙/☀️ toggle (mode persists) and a 🎨 settings panel — theme selects per mode over all themes, "create custom theme" (clones the currently active theme), collapsible per-theme editors with per-token color pickers, delete.
- Works identically in the Wails webview and a browser (no server dependency).

## Validation

`go test ./...`, scaffold checks, `mvp_smoke.sh`, web typecheck+build. Live browser verification via Playwright against `notrios -no-gui`: sidebar order confirmed ("All notes" first, "Trash" last); toggle flipped `--bg` to the dark palette and back; the mode survived a reload; a custom theme ("Midnight Custom") was created through the real UI, persisted, appeared in both selects, and was assigned as the dark-mode theme.
