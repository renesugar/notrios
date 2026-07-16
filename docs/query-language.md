# Search query language

One query language works everywhere: the GUI search box, the REST/MCP search APIs, and search-notebook queries.

| Example | Meaning |
|---|---|
| `apples oranges` | both words, anywhere in title or body |
| `"exact phrase"` | phrase match |
| `title:architecture` | word in the title |
| `title:"multiple words"` | phrase in the title |
| `notebook:"Work"` | limit to a notebook **and its sub-notebooks** (case-insensitive) |
| `tag:toys`, `tag:"shopping mall"` | notes with a tag |
| `author:"Alice Smith"` | imported notes by display name |
| `authorid:@alice` | imported notes by canonical account |
| `since:2026-07-01` | published/created on or after that date |
| `until:2026-07-31` | through the **end** of that day |
| `since:2026-07-13T18:42:07Z` | timestamps to the second |
| `since:14:30` | today at that time |

Details worth knowing:

- Everything combines with AND: `notebook:"Work" tag:todo quarterly`.
- Unknown `word:value` tokens are treated as literal text, so pasted URLs and things like `re:invoice` just work.
- Date-only `until:` means 23:59:59 of that day; date-only `since:` means midnight, in your local timezone.
- `since:`/`until:` compare the original published time of imported posts, falling back to the note's creation time.
- The empty query is "All notes"; the Trash search notebook uses the reserved query `is:trashed`.

Any query can be saved as a **search notebook**: give it a name (and an emoji if you like) and it appears in the sidebar. Deleting a search notebook never deletes notes — only the saved query.
