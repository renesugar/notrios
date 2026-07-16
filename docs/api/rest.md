# REST API

Everything the GUI does goes through the REST API at `/api/v1`; the machine-readable contract lives in `api/openapi.yaml` in the repository.

## The essentials

```text
POST   /api/v1/documents                 create a note (accepts notebook_id)
GET    /api/v1/documents/{id}            read (ETag = current revision)
PUT    /api/v1/documents/{id}            replace (requires If-Match or base_revision_id)
PATCH  /api/v1/documents/{id}            title changes + string-replace edits (dry-run supported)
DELETE /api/v1/documents/{id}            move to Trash (requires precondition)
POST   /api/v1/search                    {"query": "...", "limit": 25, "cursor": "..."}
```

Search accepts the [query language](../query-language.md) and returns `next_cursor` for incremental scrolling. All mutations use optimistic concurrency: send the revision you based your edit on and get `409` if someone else wrote first.

## Notebooks, tags, trash

```text
GET    /api/v1/notebooks/tree            nested sidebar tree
POST   /api/v1/notebooks                 {"name": "...", "parent_id": "...", "icon_emoji": "📥"}
PATCH  /api/v1/notebooks/{id}            rename / move / emoji
DELETE /api/v1/notebooks/{id}            notes move to Trash
GET    /api/v1/tags                      tags with note counts
POST   /api/v1/documents/{id}/tags/{tag}
POST   /api/v1/documents/{id}/notebook   {"notebook_id": "..."}
GET    /api/v1/search-notebooks          "All notes" first, "Trash" last
POST   /api/v1/search-notebooks          save a query
GET    /api/v1/trash                     list; POST /api/v1/trash/{id}/restore; DELETE /api/v1/trash/{id}
```

Name collisions return `409 name_conflict` (notebook names are case-insensitive); protected operations (deleting builtins, purging imported notes, editing Help notes) return `403`.

## Note content operations

```text
POST /api/v1/documents/{id}/append       {"text": "..."}
POST /api/v1/documents/{id}/prepend      {"text": "..."}
GET  /api/v1/documents/{id}/lines?start=10&end=20
GET  /api/v1/documents/{id}/search-in?pattern=needle
GET  /api/v1/documents/{id}/outline      markdown headings with line numbers
```

Resources (attachments), links/backlinks, and a graph endpoint round out the surface — enough to build a complete client without ever touching the database.
