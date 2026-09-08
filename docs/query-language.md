# Search query language

One query language works everywhere: the GUI search box, the REST/MCP search APIs, and search-notebook queries.

| Example | Meaning |
|---|---|
| `apples oranges` | both words, anywhere in title or body |
| `apples OR oranges` | either word (`OR` must be uppercase) |
| `(apples OR oranges) fresh` | grouping; implicit AND binds more tightly than OR |
| `-tag:private`, `-(tag:private OR tag:draft)` | exclude a field or group |
| `"exact phrase"` | phrase match |
| `title:architecture` | word in the title |
| `title:"multiple words"` | phrase in the title |
| `notebook:"Work"` | limit to a notebook **and its sub-notebooks** (case-insensitive) |
| `category:"Work"` | exact alias for `notebook:` |
| `category:"All notes"` | all current notes; removes that notebook constraint |
| `collection:"joplin"` | limit to notes that came from one place (their provenance) |
| `tag:toys`, `tag:"shopping mall"` | notes with a tag |
| `author:"Alice Smith"` | imported notes by display name |
| `authorid:@alice` | imported notes by canonical account |
| `since:2026-07-01` | published/created on or after that date |
| `until:2026-07-31` | through the **end** of that day |
| `since:2026-07-13T18:42:07Z` | timestamps to the second |
| `since:14:30` | today at that time |

**Finding the values.** `notebook:` and `collection:` take identifiers, and you
have to know which ones exist before you can narrow anything with them:

```sh
notriosctl notebooks list      # ids and names; --json for a script
notriosctl collections list    # ids, names, and how many notes name each
```

A search spans every collection unless a `collection:` term narrows it. A note
written in Notrios is in `default`; a note that arrived from somewhere else
carries the identifier its import was given.

Details worth knowing:

- Adjacent terms combine with AND: `notebook:"Work" tag:todo quarterly`.
- AND binds more tightly than uppercase OR. Lowercase `or` is searchable text.
- Prefix `-` negates the next term, field, or parenthesized group.
- Unknown `word:value` tokens are treated as literal text, so pasted URLs and things like `re:invoice` just work.
- Queries are bounded to 4,096 UTF-8 bytes, 256 tokens, and 16 parenthesis levels. Invalid expressions are rejected clearly.
- Date-only `until:` means 23:59:59 of that day; date-only `since:` means midnight, in your local timezone.
- `since:`/`until:` compare the original published time of imported posts, falling back to the note's creation time.
- The empty query is "All notes"; the Trash search notebook uses the reserved query `is:trashed`.

Any query can be saved as a **search notebook**: give it a name (and an emoji if you like) and it appears in the sidebar. Deleting a search notebook never deletes notes — only the saved query.

SQLite and optional Recoll compile the same application-owned expression tree.
Recursive category filters and standalone emoji are represented explicitly in
the Recoll projection; if Recoll cannot honor a query shape, Notrios uses the
exact SQLite result instead of weakening the expression.

## Using a query inside a note

A fenced `note-query` block runs one of these queries and renders the matching
notes in the preview:

````markdown
```note-query
query: tag:todo -tag:done
fields: notebook, updated
sort: updated
limit: 20
```
````

The `query:` line is exactly the language above — a block can find what you
could type into the search box, and nothing more. See the
[built-in GUI guide](gui.md) for the other keys and what a block does when the
query has a mistake in it.
