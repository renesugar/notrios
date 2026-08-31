#!/usr/bin/env python3
"""Strict, model-free G18g release/build evidence validator."""
from __future__ import annotations
import argparse, hashlib, json, re, sys
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlsplit

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
ROUTES = ["index.html", "installation.html", "service.html", "cli.html", "query-language.html", "selection-planning.html", "archive-v2.html", "stable-links.html", "publishing.html", "gui.html", "import-export.html", "operations.html", "api/rest.html", "api/mcp.html", "troubleshooting.html"]
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
    require(r.get("legacy_fragment_aliases")==ALIASES and r.get("g18a_section_count")==199 and r.get("required_alias_count")==2, "route/alias contract drift")
    require(b.get("schema")=="notrios.g18g.build-contract.v1" and b.get("builder")=="scripts/build_docs_site.sh" and b.get("production_source")=="docs-site/" and b.get("output_argument")=="first positional argument", "build contract drift")
    require(b.get("tool_pins")=={"hugo":"0.164.0 extended","node":"26.3.0","pagefind":"1.5.2"}, "tool pins drift")
    raw=b.get("raw_docs_help_contract",{}); require(raw=={"source":"docs/**/*.md","staging":"temporary byte-copy before Hugo publication rendering","publication_adapter":"generated CLI registry angle-placeholder escaping only","byte_equivalent":True,"help_id_equivalent":True,"help_content_equivalent":True,"idempotent":True}, "raw-doc/Help contract drift")
    search=b.get("search",{}); require(search.get("scope_marker")=="data-pagefind-body" and search.get("indexed_pages")==15 and search.get("known_query")=="Argon2id" and search.get("result_base")=="/notrios/" and search.get("excluded")==["api/index.html","search/index.html"], "search contract drift")
    off=b.get("offline_policy",{}); require(off=={"remote_runtime_assets":False,"remote_fonts":False,"local_pagefind_bundle":True,"csp_external_requests":False}, "offline policy drift")
    require(report.get("schema")=="notrios.g18g.qa-report.v1" and report.get("routes")=={"preserved":15,"g18a_sections":199,"aliases":2}, "report schema/route evidence")
    require(report.get("browser_plugin",{}).get("available") is False and report["browser_plugin"].get("fallback")=="browser_smoke.mjs", "browser fallback evidence")
    require(report.get("raw_help")=={"byte_equivalent":True,"id_equivalent":True,"content_equivalent":True,"idempotent":True}, "raw Help report evidence")
    require(report.get("status")=="complete", "report is not complete")
    require(report.get("validation")=={"validator":"passed source, built-site, and mutation gates","mutation_tests":5,"desktop":"passed","mobile":"passed"}, "validation report incomplete")
    repeat=report.get("reproducibility",{}); require(repeat.get("hugo_output_byte_identical") is True and repeat.get("pagefind_semantically_identical") is True and repeat.get("pagefind_byte_identical") is False and repeat.get("repeat_indexed_pages")==15 and repeat.get("repeat_known_query")=="pass", "reproducibility qualification drift")
    measurements = report.get("measurements", {})
    require(all(isinstance(measurements.get(key), (int, float)) and measurements[key] > 0 for key in ("output_files", "output_bytes", "html_bytes", "pagefind_bytes", "build_seconds")), "measured build/output values missing")
    require(len(m.get("mutations",[]))>=5 and {x.get("id") for x in m["mutations"]} >= {"theme-pin","route-fragment","search-scope","raw-help-equivalence","runtime-offline"}, "mutation matrix incomplete")
    return b

def local_target(route, href):
    p=urlsplit(href)
    if p.scheme or p.netloc or href.startswith(("mailto:","javascript:")): return None
    path=unquote(p.path)
    if path.startswith("/notrios/"): path=path[9:]
    elif path.startswith("/"): return "OUTSIDE_BASE",unquote(p.fragment)
    elif path: path=(Path(route).parent/path).as_posix()
    else: path=route
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
            if asset.startswith(("/notrios/","data:","#")): continue
            errors.append(f"remote asset {route} -> {asset}")
    html=list(site.rglob("*.html")); indexed=sum(page(x).body for x in html)
    if indexed != 15: errors.append(f"Pagefind body pages {indexed} != 15")
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
