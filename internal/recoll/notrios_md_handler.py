#!/usr/bin/env python3
# Notrios Markdown front-matter input handler for Recoll.
#
# Copyright 2026 Rene Sugar
# Licensed under the Apache License, Version 2.0.
#
# Written from scratch for Notrios (not derived from Recoll's rclmd.py, which
# is GPL). Recoll invokes it as a simple "exec" filter: it receives a Markdown
# file path as its single argument and prints an HTML document on stdout.
# YAML (--- ... ---) and TOML (+++ ... +++) front matter is extracted into
# <meta> fields following the mapping in RECOLL_INTEGRATION.md:
#
#   title      -> <title> and the title field
#   author     -> author            (display name)
#   author_id  -> authorid          (canonical account identity)
#   published  -> published (stored) and publishedts (integer Unix seconds)
#   tags       -> tag (repeated) and keywords (general-term searchable)
#   aliases    -> alias (repeated) and keywords
#   thread_id  -> threadid
#   reply_to   -> replyto
#   id         -> noteid
#   notebook   -> notebook
#   notebook_ancestors -> notebook (repeated, for recursive category filters)
#
# YAML parsing uses PyYAML when installed and otherwise falls back to a small
# built-in parser covering the flat scalars-and-lists subset that Notrios
# projections and common note apps emit. TOML uses the stdlib tomllib.

import html
import re
import sys
import unicodedata
from datetime import datetime, timezone

FIELD_MAP = {
    "title": "title",
    "author": "author",
    "author_id": "authorid",
    "thread_id": "threadid",
    "reply_to": "replyto",
    "id": "noteid",
    "notebook": "notebook",
    # Provenance rather than filing. A single-valued field, because a note
    # belongs to exactly one collection -- which is what distinguishes it from
    # a tag and why it is not in LIST_FIELDS below.
    "collection": "collection",
    "source_url": "sourceurl",
}

LIST_FIELDS = {
    "tags": "tag",
    "aliases": "alias",
}

MULTI_FIELDS = {
    "notebook_ancestors": "notebook",
}


def split_front_matter(text):
    """Return (front_matter, marker, body). marker is '---', '+++' or ''."""
    for marker in ("---", "+++"):
        if text.startswith(marker + "\n") or text.startswith(marker + "\r\n"):
            end = re.search(r"^%s\s*$" % re.escape(marker), text[3:], re.M)
            if end:
                start = 3
                return text[start:start + end.start()], marker, text[start + end.end():]
    return "", "", text


def parse_yaml_subset(source):
    """Minimal YAML mapping parser: scalars, quoted scalars, block lists,
    and inline [a, b] lists. Good enough for flat note front matter."""
    data = {}
    current_list_key = None
    for raw in source.splitlines():
        line = raw.rstrip()
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        stripped = line.strip()
        if stripped.startswith("- ") or stripped == "-":
            if current_list_key is not None:
                data.setdefault(current_list_key, []).append(unquote(stripped[1:].strip()))
            continue
        if ":" not in stripped:
            continue
        key, _, value = stripped.partition(":")
        key = key.strip()
        value = value.strip()
        if value == "":
            current_list_key = key
            data.setdefault(key, [])
            continue
        current_list_key = None
        if value.startswith("[") and value.endswith("]"):
            items = [unquote(v.strip()) for v in value[1:-1].split(",") if v.strip()]
            data[key] = items
        else:
            data[key] = unquote(value)
    return data


def unquote(value):
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
        return value[1:-1]
    return value


def parse_front_matter(source, marker):
    if marker == "+++":
        try:
            import tomllib

            return tomllib.loads(source)
        except Exception:
            return {}
    try:
        import yaml  # type: ignore

        data = yaml.safe_load(source)
        return data if isinstance(data, dict) else {}
    except Exception:
        return parse_yaml_subset(source)


def to_epoch(value):
    value = str(value).strip()
    if re.fullmatch(r"\d{10,13}", value):
        n = int(value)
        return n // 1000 if n > 1_000_000_000_000 else n
    normalized = value.replace("Z", "+00:00")
    for parse in (
        lambda v: datetime.fromisoformat(v),
        lambda v: datetime.strptime(v, "%Y-%m-%d %H:%M:%S"),
        lambda v: datetime.strptime(v, "%Y-%m-%d"),
    ):
        try:
            dt = parse(normalized)
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            return int(dt.timestamp())
        except ValueError:
            continue
    return None


def as_list(value):
    if value is None:
        return []
    if isinstance(value, (list, tuple)):
        return [str(v) for v in value if str(v).strip()]
    return [str(value)] if str(value).strip() else []


def meta(name, content):
    return '<meta name="%s" content="%s">' % (html.escape(name, quote=True), html.escape(str(content), quote=True))


def main():
    if len(sys.argv) != 2:
        sys.stderr.write("usage: notrios_md_handler.py <markdown-file>\n")
        return 1
    try:
        with open(sys.argv[1], "r", encoding="utf-8", errors="replace") as fh:
            text = fh.read()
    except OSError as exc:
        sys.stderr.write("notrios_md_handler: %s\n" % exc)
        return 1

    front, marker, body = split_front_matter(text)
    fields = parse_front_matter(front, marker) if marker else {}

    head = ["<html>", "<head>"]
    title = str(fields.get("title", "")).strip()
    if title:
        head.append("<title>%s</title>" % html.escape(title))
    for key, field in FIELD_MAP.items():
        value = fields.get(key)
        if value is not None and str(value).strip() and key != "title":
            head.append(meta(field, value))
    if title:
        head.append(meta("title", title))
    published = fields.get("published")
    if published is not None and str(published).strip():
        head.append(meta("published", published))
        epoch = to_epoch(published)
        if epoch is not None:
            head.append(meta("publishedts", epoch))
    keywords = []
    # Notebook ancestry is indexed only in the notebook field. It must not make
    # an unqualified text query match a parent notebook name.
    for key, field in MULTI_FIELDS.items():
        for item in as_list(fields.get(key)):
            head.append(meta(field, item))
    for key, field in LIST_FIELDS.items():
        for item in as_list(fields.get(key)):
            head.append(meta(field, item))
            keywords.append(item)
    if keywords:
        # Duplicate tags/aliases into keywords so unqualified searches match
        # notes whose words occur only in their tags.
        head.append(meta("keywords", ", ".join(keywords)))
    # Recoll/Xapian word tokenizers discard emoji and other Unicode symbols.
    # Index stable codepoint keys so standalone emoji queries stay exact.
    symbol_keys = []
    seen_symbols = set()
    for character in title + "\n" + body:
        if unicodedata.category(character) == "So":
            key = "u%x" % ord(character)
            if key not in seen_symbols:
                seen_symbols.add(key)
                symbol_keys.append(key)
    for key in symbol_keys:
        head.append(meta("emoji", key))
    head.append('<meta http-equiv="Content-Type" content="text/html; charset=utf-8">')
    head.append("</head>")

    out = "\n".join(head) + "\n<body><pre>" + html.escape(body) + "</pre></body></html>\n"
    sys.stdout.write(out)
    return 0


if __name__ == "__main__":
    sys.exit(main())
