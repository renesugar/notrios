#!/usr/bin/env python3
"""Native-window layout-resize verification for the Notrios workspace.

Playwright's page.set_viewport_size() emulates a fixed canvas inside the
browser window: screenshots look right while the real window — the thing a
Wails desktop build or a user actually resizes — never changes, so layout
bugs tied to native resize events stay invisible. This script removes the
emulation and verifies the geometry programmatically:

1. Chromium is launched headed (under Xvfb/Openbox) with the target size
   passed as native flags and ``viewport=None``, binding the page viewport
   to the true X window boundaries.
2. Mid-test resizes go through ``xdotool windowsize`` — a real X11 resize of
   the browser window, the same event path a desktop window manager uses —
   never through set_viewport_size().
3. After every resize the pane geometry is read from the DOM (bounding
   client rects) and asserted: the four panes fill the window's width and
   height, the body does not overflow, and the Markdown editor and preview
   come out the same width after a window resize.

Usage:
    Xvfb :99 -screen 0 2000x1200x24 & DISPLAY=:99 openbox &
    DISPLAY=:99 python3 scripts/verify_layout_resize.py --url http://127.0.0.1:8080

Requires: pip install playwright; a Chromium/Chrome binary (system
google-chrome is used when the Playwright browser download is absent);
xdotool; a running Notrios service serving the built web UI at --url.
"""

from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
import time

from playwright.sync_api import sync_playwright

# Keep in sync with web/src/panes.ts.
SPLITTER_WIDTH = 6
MIN_WORKSPACE_WIDTH = 966
# Below this window width the equal editor/preview split hits pane minimums
# (sidebar 220 + search 320 + 3*6 splitters + 2*280), so equality is only
# asserted above it.
EQUAL_SPLIT_MIN_WIDTH = 220 + 320 + 3 * SPLITTER_WIDTH + 2 * 280

METRICS_JS = """() => {
  const rect = (sel) => {
    const el = document.querySelector(sel);
    if (!el) return null;
    const b = el.getBoundingClientRect();
    return { x: b.x, y: b.y, width: b.width, height: b.height };
  };
  return {
    window: { width: window.innerWidth, height: window.innerHeight },
    workspace: rect('.workspace'),
    sidebar: rect('.sidebar-pane'),
    search: rect('.search-pane'),
    editor: rect('.editor-pane'),
    preview: rect('.preview-pane'),
    bodyOverflowX: document.documentElement.scrollWidth > document.documentElement.clientWidth,
    bodyOverflowY: document.documentElement.scrollHeight > document.documentElement.clientHeight,
  };
}"""


def resize_browser_window(width: int, height: int) -> None:
    """Force a native X11 resize of the browser window running Notrios."""
    subprocess.run(
        ["xdotool", "search", "--onlyvisible", "--name", "Notrios", "windowsize", str(width), str(height)],
        check=True,
    )
    # Give the window manager, the browser, and the flexbox/ResizeObserver
    # layout logic a moment to process the resize event.
    time.sleep(0.8)


def read_metrics(page) -> dict:
    metrics = page.evaluate(METRICS_JS)
    for key in ("workspace", "sidebar", "search", "editor", "preview"):
        if metrics[key] is None:
            raise AssertionError(f"CRITICAL: pane {key!r} not found in DOM")
    return metrics


def print_metrics(label: str, m: dict) -> None:
    w = m["window"]
    print(
        f"[{label}] window {w['width']}x{w['height']} | "
        f"sidebar {m['sidebar']['width']:.0f}px | search {m['search']['width']:.0f}px | "
        f"editor {m['editor']['width']:.0f}x{m['editor']['height']:.0f}px | "
        f"preview {m['preview']['width']:.0f}x{m['preview']['height']:.0f}px | "
        f"workspace {m['workspace']['width']:.0f}x{m['workspace']['height']:.0f}px | "
        f"body overflow x={m['bodyOverflowX']} y={m['bodyOverflowY']}"
    )


def assert_layout(m: dict, resized: bool, tolerance: float = 2.0) -> list[str]:
    """Return a list of failed assertions (empty = layout is correct)."""
    failures: list[str] = []
    win_w = m["window"]["width"]
    win_h = m["window"]["height"]
    panes = [m["sidebar"], m["search"], m["editor"], m["preview"]]

    # Horizontal fill: the four panes plus three splitters span the window.
    total = sum(p["width"] for p in panes) + 3 * SPLITTER_WIDTH
    if win_w >= MIN_WORKSPACE_WIDTH and abs(total - win_w) > tolerance:
        failures.append(f"panes+splitters span {total:.0f}px, window is {win_w}px — workspace does not fill the width")

    # Vertical fill: the workspace's bottom edge reaches the bottom of the
    # window (its top sits below the header) and every pane fills its height.
    workspace_bottom = m["workspace"]["y"] + m["workspace"]["height"]
    if abs(workspace_bottom - win_h) > tolerance:
        failures.append(f"workspace bottom edge at {workspace_bottom:.0f}px, window height is {win_h}px")
    for name in ("sidebar", "search", "editor", "preview"):
        if abs(m[name]["height"] - m["workspace"]["height"]) > tolerance:
            failures.append(f"{name} height {m[name]['height']:.0f}px != workspace height {m['workspace']['height']:.0f}px")

    # No document-level overflow at or above the minimum workspace width.
    if win_w >= MIN_WORKSPACE_WIDTH and (m["bodyOverflowX"] or m["bodyOverflowY"]):
        failures.append("document body overflows the window")

    # After a native window resize the editor and preview split equally.
    if resized and win_w >= EQUAL_SPLIT_MIN_WIDTH:
        if abs(m["editor"]["width"] - m["preview"]["width"]) > 1.0:
            failures.append(
                f"editor {m['editor']['width']:.0f}px != preview {m['preview']['width']:.0f}px after window resize"
            )

    # The prompt's canary: at large sizes the editor must have expanded.
    if win_w >= 1900 and m["editor"]["width"] <= 600:
        failures.append("CRITICAL: Markdown editor panel failed to expand with window scaling!")

    return failures


def find_browser_executable() -> str | None:
    """Prefer the Playwright-managed Chromium; fall back to system Chrome."""
    for candidate in ("google-chrome", "google-chrome-stable", "chromium", "chromium-browser"):
        path = shutil.which(candidate)
        if path:
            return path
    return None


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--url", default="http://127.0.0.1:8080", help="Notrios web UI URL (service must be running)")
    parser.add_argument(
        "--sizes",
        default="1024x768,1400x900,1920x1080",
        help="comma-separated WxH resize cycle; the first size is the launch size",
    )
    args = parser.parse_args()

    sizes = []
    for token in args.sizes.split(","):
        w, _, h = token.strip().partition("x")
        sizes.append((int(w), int(h)))
    launch_w, launch_h = sizes[0]

    all_failures: list[str] = []
    with sync_playwright() as p:
        launch_kwargs = {
            # Headed: the browser must be a real window under Xvfb/Openbox so
            # native X11 resizes reach it.
            "headless": False,
            "args": [f"--window-size={launch_w},{launch_h}", "--window-position=0,0"],
        }
        try:
            browser = p.chromium.launch(**launch_kwargs)
        except Exception:
            executable = find_browser_executable()
            if not executable:
                print("No Playwright Chromium and no system Chrome/Chromium found", file=sys.stderr)
                return 2
            browser = p.chromium.launch(executable_path=executable, **launch_kwargs)

        # Crucial: no_viewport (Python's spelling of viewport=None) stops
        # Playwright from emulating a fixed canvas; the page viewport now
        # tracks the real window size.
        context = browser.new_context(no_viewport=True)
        page = context.new_page()
        page.goto(args.url, wait_until="networkidle")
        page.wait_for_selector(".workspace")
        time.sleep(0.5)

        metrics = read_metrics(page)
        print_metrics(f"launch {launch_w}x{launch_h}", metrics)
        failures = assert_layout(metrics, resized=False)
        all_failures += [f"launch {launch_w}x{launch_h}: {f}" for f in failures]

        for width, height in sizes[1:]:
            resize_browser_window(width, height)
            metrics = read_metrics(page)
            print_metrics(f"resize {width}x{height}", metrics)
            failures = assert_layout(metrics, resized=True)
            all_failures += [f"resize {width}x{height}: {f}" for f in failures]

        browser.close()

    if all_failures:
        print("\nFAIL:")
        for failure in all_failures:
            print(f"  - {failure}")
        return 1
    print("\nPASS: the workspace fills the real window at every size and the editor/preview split equally after resizes.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
