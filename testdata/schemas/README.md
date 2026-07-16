# Derived JSON Schemas

JSON Schemas (https://json-schema.org/) for import source formats, derived
from **synthetic** sample data with genson (`uvx genson sample.json`). No real
user exports are stored in this repository.

- `twitter-tweets.schema.json` — the tweet entries in a Twitter/X archive
  `data/tweets.js` (`window.YTD.tweets.part0`), covering the field subset the
  importer reads (`internal/importers/twitter`).
- `twitter-account.schema.json` — the `data/account.js` account entry.
- `chatgpt-conversations.schema.json` — ChatGPT export `conversations.json` (mapping-tree conversations; field subset read by `internal/importers/chatgpt`).
- `claude-conversations.schema.json` — Claude export `conversations.json` (flat `chat_messages`; field subset read by `internal/importers/claude`).

Regenerate after extending a fixture:

```bash
uvx genson sample.json | python3 -m json.tool > testdata/schemas/<name>.schema.json
```
