#!/usr/bin/env python3
"""Validate G18b's source closure and, optionally, a built prototype site."""

from __future__ import annotations

import argparse
import hashlib
from html.parser import HTMLParser
import json
from pathlib import Path
import tempfile
from urllib.parse import unquote, urlsplit

import build_prototype


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class Document(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.ids: set[str] = set()
        self.hrefs: list[str] = []
        self.asset_urls: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        values = dict(attrs)
        if values.get("id"):
            self.ids.add(values["id"] or "")
        if tag == "a" and values.get("href"):
            self.hrefs.append(values["href"] or "")
        if tag in {"script", "img", "source", "video", "audio", "iframe"} and values.get("src"):
            self.asset_urls.append(values["src"] or "")
        if tag == "link" and values.get("href") and values.get("rel") in {
            "stylesheet",
            "icon",
            "preload",
            "modulepreload",
        }:
            self.asset_urls.append(values["href"] or "")


def load_json(name: str) -> dict[str, object]:
    return json.loads((HERE / name).read_text(encoding="utf-8"))


def parse_html(path: Path) -> Document:
    parsed = Document()
    parsed.feed(path.read_text(encoding="utf-8"))
    return parsed


def theme_manifest(root: Path) -> tuple[int, int, str]:
    files = sorted(path for path in root.rglob("*") if path.is_file())
    aggregate = hashlib.sha256()
    total = 0
    for path in files:
        relative = path.relative_to(root).as_posix()
        file_hash = hashlib.sha256(path.read_bytes()).hexdigest()
        aggregate.update(f"{file_hash}  {relative}\n".encode())
        total += path.stat().st_size
    return len(files), total, aggregate.hexdigest()


def route_for_doc(path: str) -> str:
    relative = path.removeprefix("docs/").removesuffix(".md")
    return "index.html" if relative == "index" else relative + ".html"


def local_target(route: str, href: str) -> tuple[str, str] | None:
    parsed = urlsplit(href)
    if parsed.scheme or parsed.netloc or href.startswith(("mailto:", "notrios:")):
        return None
    path = unquote(parsed.path)
    if path.startswith("/notrios/"):
        path = path[len("/notrios/") :]
    elif path.startswith("/"):
        return "OUTSIDE_BASE:" + path, unquote(parsed.fragment)
    elif path:
        path = (Path(route).parent / path).as_posix()
    else:
        path = route
    if path in ("", "."):
        path = "index.html"
    if path.endswith("/"):
        path += "index.html"
    return path, unquote(parsed.fragment)


def validate_source() -> list[str]:
    errors: list[str] = []
    site = load_json("SITE_CONTRACT.json")
    delivery = load_json("DELIVERY_OPTIONS.json")
    provenance = load_json("THEME_PROVENANCE.json")
    request_inventory = load_json("REQUEST_INVENTORY.json")
    license_bill = load_json("LICENSE_BILL.json")

    expected_schemas = {
        "SITE_CONTRACT.json": (site, "notrios.g18b.site-contract.v1"),
        "DELIVERY_OPTIONS.json": (delivery, "notrios.g18b.delivery-options.v1"),
        "THEME_PROVENANCE.json": (provenance, "notrios.g18b.theme-provenance.v1"),
        "REQUEST_INVENTORY.json": (request_inventory, "notrios.g18b.request-inventory.v1"),
        "LICENSE_BILL.json": (license_bill, "notrios.g18b.license-bill.v1"),
    }
    for name, (document, schema) in expected_schemas.items():
        if document.get("schema") != schema:
            errors.append(f"{name}: unexpected schema")

    if delivery.get("selected") != "minimal-vendored-source":
        errors.append("delivery option is not the approved non-blocking default")
    # The base URL is read from the site's own configuration and compared with
    # what the contract records, rather than compared with a literal here. A
    # literal made this check a second place to edit, and a check that has to be
    # edited to keep passing is one that gets edited without being read: the
    # placeholder it pinned survived from G18b to v0.9 I10 precisely because
    # nothing ever compared it to the site.
    configured = hugo_base_url(ROOT / "docs-site" / "hugo.toml")
    if configured is None:
        errors.append("docs-site/hugo.toml declares no baseURL")
    elif site.get("base_url") != configured:
        errors.append(
            f"site base URL drifted: hugo.toml says {configured!r}, "
            f"SITE_CONTRACT.json records {site.get('base_url')!r}")
    if site.get("search", {}).get("selected_backend") != "pagefind":
        errors.append("static Pagefind selection drifted")

    snapshot = ROOT / str(provenance["snapshot_root"])
    count, size, aggregate = theme_manifest(snapshot)
    if count != provenance.get("file_count"):
        errors.append(f"theme file count: {count} != {provenance.get('file_count')}")
    if size != provenance.get("byte_count"):
        errors.append(f"theme byte count: {size} != {provenance.get('byte_count')}")
    if aggregate != provenance.get("manifest_sha256"):
        errors.append("theme manifest SHA-256 drifted")
    if not (snapshot / "LICENSE").is_file():
        errors.append("vendored Ledger LICENSE is missing")

    with tempfile.TemporaryDirectory(prefix="notrios-g18b-source-") as temporary:
        staged = Path(temporary) / "site"
        copied = build_prototype.stage_source(staged)
        for source, target in copied:
            if source.read_bytes() != target.read_bytes():
                errors.append(f"staged docs bytes changed: {source.relative_to(ROOT)}")

    return errors


def validate_site(site_root: Path) -> list[str]:
    errors: list[str] = []
    contract = load_json("SITE_CONTRACT.json")
    inventory = json.loads(
        (ROOT / "performance/v0.7-g18a/INVENTORY.json").read_text(encoding="utf-8")
    )
    parsed: dict[str, Document] = {}
    for route in contract["urls"]["preserved_routes"]:
        path = site_root / route
        if not path.is_file():
            errors.append(f"missing preserved route: {route}")
            continue
        parsed[route] = parse_html(path)

    for item in inventory["documents"]:
        route = route_for_doc(item["path"])
        if route not in parsed:
            continue
        for section in item["sections"]:
            if section["id"] not in parsed[route].ids:
                errors.append(f"missing G18a section: {route}#{section['id']}")

    aliases = contract["urls"]["legacy_fragment_aliases"]
    for old in aliases:
        route, fragment = old.split("#", 1)
        if route not in parsed or fragment not in parsed[route].ids:
            errors.append(f"missing legacy fragment alias: {old}")

    for route, document in parsed.items():
        for href in document.hrefs:
            resolved = local_target(route, href)
            if resolved is None:
                continue
            target, fragment = resolved
            if target.startswith("OUTSIDE_BASE:"):
                errors.append(f"base-path escape: {route} -> {href}")
                continue
            if target.endswith(".md"):
                errors.append(f"Markdown link survived render: {route} -> {href}")
            target_path = site_root / target
            if not target_path.is_file():
                errors.append(f"broken target: {route} -> {href}")
            elif fragment and fragment not in parse_html(target_path).ids:
                errors.append(f"broken fragment: {route} -> {href}")
        for url in document.asset_urls:
            if url.startswith(("data:", "/notrios/", "#")):
                continue
            errors.append(f"non-local runtime asset: {route} -> {url}")

    all_html = list(site_root.rglob("*.html"))
    indexed = [path for path in all_html if "data-pagefind-body" in path.read_text(encoding="utf-8")]
    if len(indexed) != contract["search"]["indexed_pages"]:
        errors.append(f"Pagefind scope count: {len(indexed)} != {contract['search']['indexed_pages']}")
    for excluded in contract["search"]["excluded_generated_pages"]:
        path = site_root / excluded
        if not path.is_file():
            errors.append(f"missing generated route: {excluded}")
        elif "data-pagefind-body" in path.read_text(encoding="utf-8"):
            errors.append(f"generated furniture entered Pagefind scope: {excluded}")

    joined = "\n".join(path.read_text(encoding="utf-8") for path in all_html)
    if "fonts.googleapis.com" in joined or "fonts.gstatic.com" in joined:
        errors.append("Google Fonts request survived googleFonts=false")
    if not (site_root / "pagefind/pagefind.js").is_file():
        errors.append("Pagefind bundle missing")
    return errors


def hugo_base_url(path: pathlib.Path) -> str | None:
    """Read baseURL out of hugo.toml without a TOML parser.

    The file is a handful of top-level keys and this needs one of them; adding a
    dependency to the offline validator to read a single line would cost more
    than it explains.
    """
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if stripped.startswith("baseURL"):
            _, _, value = stripped.partition("=")
            return value.strip().strip("'\"")
    return None


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--site", type=Path)
    args = parser.parse_args()
    errors = validate_source()
    if args.site:
        errors.extend(validate_site(args.site.resolve()))
    if errors:
        for error in errors:
            print(f"ERROR: {error}")
        raise SystemExit(1)
    suffix = f" and built site {args.site.resolve()}" if args.site else ""
    print(f"G18b evidence and source closure passed{suffix}.")


if __name__ == "__main__":
    main()
