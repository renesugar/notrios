#!/usr/bin/env python3
"""Strict, model-free G18g release/build evidence validator."""
from __future__ import annotations
import argparse, hashlib, json, re, sys
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlsplit

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
# 15 -> 18 in v0.8: features.html and the two journey catalogues. The list is
# frozen rather than discovered so that a page appearing on the published site
# is a decision somebody made -- and because this gate only runs at packaging
# time, "somebody" turned out to be nobody for a month. v0.8 H13 moved the
# gates that need no browser into `make validate`; this one still needs
# Pagefind and Hugo, so it stays here.
# Public routes, pinned so that a page appearing on the site -- or a published
# URL quietly disappearing -- is a decision somebody made. "configuration.html"
# was added in v1.0 J12 with docs/configuration.md; nothing here has ever been
# removed, because these are addresses other people may have linked to.
ROUTES = ["index.html", "installation.html", "configuration.html", "service.html", "cli.html", "query-language.html", "selection-planning.html", "archive-v2.html", "stable-links.html", "publishing.html", "gui.html", "import-export.html", "operations.html", "api/rest.html", "api/mcp.html", "troubleshooting.html", "features.html", "journeys-cli.html", "journeys-gui.html"]
ALIASES = {"service.html#configuration": "service.html#configuration-reference", "stable-links.html#linking-to-a-block-not-just-a-note": "stable-links.html#linking-to-a-section-or-a-block"}

class EvidenceError(ValueError): pass
def require(ok, msg):
    if not ok: raise EvidenceError(msg)
def load(name): return json.loads((HERE / name).read_text(encoding="utf-8"))
def sha(data): return hashlib.sha256(data).hexdigest()

class Page(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True); self.ids=set(); self.hrefs=[]; self.assets=[]; self.body=0
    def handle_starttag(self, tag, attrs):
        a=dict(attrs)
        if a.get("id"): self.ids.add(a["id"])
        if tag == "a" and a.get("href"): self.hrefs.append(a["href"])
        if "data-pagefind-body" in a: self.body += 1
        if tag in {"script","img","source","video","audio","iframe"} and a.get("src"): self.assets.append(a["src"])
        if tag == "link" and a.get("href") and a.get("rel") in {"stylesheet","icon","preload","modulepreload"}: self.assets.append(a["href"])

def page(path):
    p=Page(); p.feed(path.read_text(encoding="utf-8")); return p

def theme_manifest(root):
    rows=[]; total=0
    for f in sorted(x for x in root.rglob("*") if x.is_file()):
        rel=f.relative_to(root).as_posix(); b=f.read_bytes(); rows.append((rel,sha(b),len(b))); total += len(b)
    h=hashlib.sha256()
    for rel,digest,_ in rows: h.update(f"{digest}  {rel}\n".encode())
    return rows,total,h.hexdigest()

def validate_theme_pair(production, frozen, provenance):
    require(production.is_dir(), "production docs-site theme missing")
    frozen_rows, frozen_total, frozen_manifest = theme_manifest(frozen)
    production_rows, production_total, production_manifest = theme_manifest(production)
    require(len(frozen_rows)==44 and frozen_total==173947 and frozen_manifest=="2897566a3861c1a92f6f83d2606d8605c085d4b63d3f67adbef4954c55880025", "frozen theme manifest drift")
    require(frozen_rows == production_rows, "production theme differs from frozen snapshot file or hash")
    require(production_manifest == frozen_manifest and production_total == frozen_total, "production theme manifest drift")
    require((production/"LICENSE").is_file(), "production theme LICENSE missing")
    require(provenance.get("schema")=="notrios.g18g.theme-provenance.v1", "production theme provenance schema")
    require(provenance.get("upstream_commit")=="f9d28ea297427890ecffa31fa74caa9ee385d9f5", "production theme upstream commit drift")
    require((provenance.get("file_count"),provenance.get("byte_count"),provenance.get("manifest_sha256"))==(44,173947,frozen_manifest), "production theme provenance hash values")

def source_checks(root=ROOT, bundle=HERE):
    def read(name): return json.loads((bundle/name).read_text(encoding="utf-8"))
    t=read("THEME_PROVENANCE.json"); r=read("ROUTES.json"); b=read("BUILD_CONTRACT.json"); m=read("MUTATION_MATRIX.json"); report=read("REPORT.json")
    require(t.get("schema")=="notrios.g18g.theme-provenance.v1", "theme schema")
    require(t.get("upstream_commit")=="f9d28ea297427890ecffa31fa74caa9ee385d9f5", "theme upstream commit drift")
    frozen=root / "performance/v0.7-g18b/prototype/themes/hugo-theme-ledger"
    production=root / "docs-site/themes/hugo-theme-ledger"
    validate_theme_pair(production, frozen, json.loads((production.parent.parent/"THEME_PROVENANCE.json").read_text(encoding="utf-8")))
    require(r.get("schema")=="notrios.g18g.routes.v1" and r.get("preserved_routes")==ROUTES, "preserved route set drift")
    require(r.get("legacy_fragment_aliases")==ALIASES and r.get("g18a_section_count")==267 and r.get("required_alias_count")==2, "route/alias contract drift")
    require(b.get("schema")=="notrios.g18g.build-contract.v1" and b.get("builder")=="scripts/build_docs_site.sh" and b.get("production_source")=="docs-site/" and b.get("output_argument")=="first positional argument", "build contract drift")
    require(b.get("tool_pins")=={"hugo":"0.164.0 extended","node":"26.3.0","pagefind":"1.5.2"}, "tool pins drift")
    raw=b.get("raw_docs_help_contract",{}); require(raw=={"source":"docs/**/*.md","staging":"temporary byte-copy before Hugo publication rendering","publication_adapter":"generated CLI registry angle-placeholder escaping only","byte_equivalent":True,"help_id_equivalent":True,"help_content_equivalent":True,"idempotent":True}, "raw-doc/Help contract drift")
    search=b.get("search",{}); require(search.get("scope_marker")=="data-pagefind-body" and search.get("indexed_pages")==18 and search.get("known_query")=="Argon2id" and search.get("result_base")==BASE_PATH and search.get("excluded")==["api/index.html","search/index.html"], "search contract drift")
    off=b.get("offline_policy",{}); require(off=={"remote_runtime_assets":False,"remote_fonts":False,"local_pagefind_bundle":True,"csp_external_requests":False}, "offline policy drift")
    require(report.get("schema")=="notrios.g18g.qa-report.v1" and report.get("routes")=={"preserved":18,"g18a_sections":267,"aliases":2}, "report schema/route evidence")
    require(report.get("browser_plugin",{}).get("available") is False and report["browser_plugin"].get("fallback")=="browser_smoke.mjs", "browser fallback evidence")
    require(report.get("raw_help")=={"byte_equivalent":True,"id_equivalent":True,"content_equivalent":True,"idempotent":True}, "raw Help report evidence")
    require(report.get("status")=="complete", "report is not complete")
    require(report.get("validation")=={"validator":"passed source, built-site, and mutation gates","mutation_tests":5,"desktop":"passed","mobile":"passed"}, "validation report incomplete")
    repeat=report.get("reproducibility",{}); require(repeat.get("hugo_output_byte_identical") is True and repeat.get("pagefind_semantically_identical") is True and repeat.get("pagefind_byte_identical") is False and repeat.get("repeat_indexed_pages")==18 and repeat.get("repeat_known_query")=="pass", "reproducibility qualification drift")
    measurements = report.get("measurements", {})
    require(all(isinstance(measurements.get(key), (int, float)) and measurements[key] > 0 for key in ("output_files", "output_bytes", "html_bytes", "pagefind_bytes", "build_seconds")), "measured build/output values missing")
    require(len(m.get("mutations",[]))>=5 and {x.get("id") for x in m["mutations"]} >= {"theme-pin","route-fragment","search-scope","raw-help-equivalence","runtime-offline"}, "mutation matrix incomplete")
    return b

def site_base_path():
    """The path the site is served under, from the site's own configuration.

    Three checks below used to hardcode "/notrios/": the search result base, the
    prefix stripped from absolute links, and the prefix that marked an asset as
    site-local. That was right while the base URL was
    https://example.github.io/notrios/ and wrong the moment it stopped being --
    and it was wrong in the worst direction, because a root-served site emits
    "/css/x.css" and the asset check then read a leading slash as a filesystem
    path and reported that the site did not carry a file it plainly carried.

    Deriving it means the base path can only be wrong in one place, and that
    place is the file Hugo actually reads.
    """
    for line in (ROOT / "docs-site" / "hugo.toml").read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if stripped.startswith("baseURL"):
            _, _, value = stripped.partition("=")
            path = urlsplit(value.strip().strip("'\"")).path or "/"
            return path if path.endswith("/") else path + "/"
    raise SystemExit("docs-site/hugo.toml declares no baseURL")


BASE_PATH = site_base_path()


def local_target(route, href):
    p=urlsplit(href)
    if p.scheme or p.netloc or href.startswith(("mailto:","javascript:")): return None
    path=unquote(p.path)
    if BASE_PATH != "/" and path.startswith(BASE_PATH): path=path[len(BASE_PATH):]
    elif path.startswith("/"):
        # Root-served sites address their own pages from "/", so an absolute
        # path is only an escape when the site lives under a prefix.
        if BASE_PATH != "/": return "OUTSIDE_BASE",unquote(p.fragment)
        path=path[1:]
    elif path: path=(Path(route).parent/path).as_posix()
    else: path=route
    # KNOWN LIMITATION, in two parts. A *relative* directory link resolves to a
    # directory rather than to its index.html, so it is reported as a broken
    # link that is not broken:
    #
    #     "/search/"   from index.html   -> "search/index.html"   correct
    #     "./search/"  from index.html   -> "search"              directory
    #     "../search/" from api/mcp.html -> "api/../search"        directory
    #
    # `as_posix()` above normalises the trailing slash away, so the test below
    # cannot see it -- and, as the third line shows, it does not normalise ".."
    # either. Whoever fixes this needs both: remember the trailing slash before
    # normalising, and resolve the path lexically, or the route strings will not
    # match the keys `parsed` is indexed by even where the filesystem would
    # happily resolve them. Absolute links are unaffected: they skip the branch
    # above and keep their slash.
    #
    # Nothing emits relative links today, so this is latent. It surfaced in v0.9
    # I10 when `relativeURLs = true` was tried for dual-address compatibility
    # (notrios.com and the renesugar.github.io/notrios/ fallback from one build):
    # the validator reported 54 broken links against a site whose pages all
    # existed. The fix is to remember the trailing slash before normalising --
    # see the skipped test in test_validate_evidence.py, which describes the
    # behaviour that should hold.
    if path.endswith("/"): path += "index.html"
    return path or "index.html",unquote(p.fragment)

def validate_site(site):
    errors=[]; parsed={}
    for route in ROUTES:
        p=site/route
        if not p.is_file(): errors.append(f"missing route {route}"); continue
        parsed[route]=page(p)
    inv=json.loads((ROOT/"performance/v0.7-g18a/INVENTORY.json").read_text())
    seen=0
    for item in inv["documents"]:
        route=item["path"].removeprefix("docs/").removesuffix(".md")+".html"
        if route=="index.html": route="index.html"
        if route not in parsed: continue
        for section in item["sections"]:
            seen += 1
            if section["id"] not in parsed[route].ids: errors.append(f"missing section {route}#{section['id']}")
    expected_sections=sum(len(item["sections"]) for item in inv["documents"])
    if seen != expected_sections: errors.append(f"G18a section inventory count {seen} != {expected_sections}")
    if expected_sections < 199: errors.append(f"G18a section inventory regressed below frozen baseline: {expected_sections}")
    for old,new in ALIASES.items():
        route,frag=old.split("#",1); _,target=new.split("#",1)
        if route not in parsed or frag not in parsed[route].ids or target not in parsed[route].ids: errors.append(f"missing alias {old}")
    for route,doc in parsed.items():
        for href in doc.hrefs:
            resolved=local_target(route,href)
            if not resolved: continue
            target,frag=resolved
            if target=="OUTSIDE_BASE": errors.append(f"base escape {route} -> {href}"); continue
            if target.endswith(".md"): errors.append(f"markdown link {route} -> {href}")
            p=site/target
            if not p.is_file(): errors.append(f"broken link {route} -> {href}")
            elif frag and frag not in page(p).ids: errors.append(f"broken fragment {route} -> {href}")
        for asset in doc.assets:
            if asset.startswith(("data:","#")): continue
            if BASE_PATH != "/" and asset.startswith(BASE_PATH): continue
            if BASE_PATH == "/" and asset.startswith("/"):
                # Site-root-absolute. Resolved against the site rather than the
                # filesystem, which is what joining a leading slash would do.
                if (site / asset[1:]).is_file(): continue
                errors.append(f"asset the site does not carry {route} -> {asset}")
                continue
            # A relative reference that resolves to a file the site carries is
            # not a remote asset, and requiring the absolute prefix said it was.
            # v0.8's illustrated interface journeys write `images/journeys/x.png`
            # in `docs/`, because that path also has to be right when the same
            # Markdown is read as a file and seeded into the Help notebook -- the
            # build contract copies docs/ byte-for-byte, so the site cannot
            # rewrite it. What this check is for is that nothing is fetched from
            # off-site, and resolving the path proves that more strongly than
            # matching a prefix does.
            if "://" not in asset and not asset.startswith("//"):
                resolved = (site / route).parent / asset
                if resolved.is_file():
                    continue
                errors.append(f"asset the site does not carry {route} -> {asset}")
                continue
            errors.append(f"remote asset {route} -> {asset}")
    html=list(site.rglob("*.html")); indexed=sum(page(x).body for x in html)
    # 18 -> 19 in v1.0 J12: docs/configuration.md. A page on the site that
    # Pagefind does not index is a page the offline search cannot find, which is
    # the failure this count exists to catch.
    if indexed != 19: errors.append(f"Pagefind body pages {indexed} != 19")
    for excluded in ("api/index.html","search/index.html"):
        p=site/excluded
        if p.is_file() and page(p).body: errors.append(f"generated page indexed {excluded}")
    if not (site/"pagefind/pagefind.js").is_file(): errors.append("local Pagefind bundle missing")
    cli=(site/"cli.html").read_text(encoding="utf-8") if (site/"cli.html").is_file() else ""
    service=(site/"service.html").read_text(encoding="utf-8") if (site/"service.html").is_file() else ""
    if "&lt;raw-export-dir>" not in cli: errors.append("generated CLI angle placeholder was dropped")
    if "<pre" not in cli or "<code" not in cli: errors.append("semantic code rendering missing")
    if "<table" not in service: errors.append("semantic table rendering missing")
    return errors

def main():
    ap=argparse.ArgumentParser(); ap.add_argument("--site",type=Path); args=ap.parse_args()
    try: source_checks(); errors=validate_site(args.site.resolve()) if args.site else []
    except (EvidenceError,KeyError,TypeError,FileNotFoundError) as e: print(f"ERROR: {e}"); return 1
    for e in errors: print("ERROR:",e)
    if errors: return 1
    print("G18g source evidence passed" + (f" and site {args.site.resolve()}" if args.site else "")); return 0
if __name__=="__main__": sys.exit(main())
