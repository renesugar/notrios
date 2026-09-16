# v1.0 J25: import a Twitter/X archive as downloaded, completely, at its real size

The archive is the owner's own, and private. This record holds only counts,
sizes and timings: no post text, names, media or IDs from it. Every test fixture
is synthetic.

## What was found (2026-09-15)

The importer from v0.2 (`internal/importers/twitter`) had never run on a real
archive. The owner's archive, `twitter-2026-08-18-….zip`, is 3,320,436,070
bytes with 15,088 entries. Read against it, the importer had three problems:

| | what the archive holds | what the importer did |
|---|---|---|
| **format** | a ZIP, as X delivers it | accepted an extracted folder only |
| **posts** | `data/tweets.js` 55,339, `tweets-part1.js` 53,822, `tweets-part2.js` 52,368, `tweets-part3.js` 14,894: **176,423**, matching `tweet-headers.js` | read `tweets.js` only: **121,084 posts (68.6%) silently left out** |
| **memory** | about 105 MB of JSON per part | read each file whole and decoded it in one piece |

Also in the archive:
- **Media:** `data/tweets_media/` holds 8,736 files, 3.31 GB.
- **Other post files:** `community-tweet.js` has 1 post, `deleted-tweets.js`
  has 6, and `note-tweet.js` is empty.

**The owner decided (2026-09-15):** community posts are imported; deleted posts
are not, and are counted in the report.

## Before the change

**Synthetic, on the unchanged importer.** Two throwaway tests used only the
report fields that existed before J25. Both failed, and were removed once
recorded:

```
split folder: tweets_seen=2 err=<nil>, want 5 posts from three parts
downloaded zip: tweets_seen=0 err=no tweets.js/tweet.js found under ".../archive.zip"; expected an extracted Twitter/X archive, want 1
```

**The real archive, on the unchanged importer.** `notriosctl` was built at
`bda7518` and dry-run against the archive's `account.js` and four post files,
extracted to disk:
- **Found:** `tweets_seen` 55,339, 8,316 threads recovered, no warning.
- **Cost:** 5.34 s, peak RSS 344 MB.

## J25-A: the downloaded ZIP and every part

- **One source interface.** `archive.go` puts a single interface over an
  extracted folder and the ZIP read in place.
  - A ZIP is opened with `archive/zip`, and its entries are looked up by name.
    Nothing is extracted, and no entry name is joined onto the filesystem.
  - Since Go 1.20, a ZIP with a `..` or absolute name opens with
    `zip.ErrInsecurePath` and a usable reader. The importer accepts that
    reader, and refuses such entries one by one.
- **Every post file, in part order.** The post files are `tweets.js` and every
  `tweets-partN.js` in part order, or an older archive's `tweet.js`.
- **The other post files.**
  - `community-tweet.js` is imported.
  - `deleted-tweets.js` is counted and not imported.
  - `tweet-headers.js` is counted and compared with the posts found. A
    difference is a warning.
- **Duplicates.** A post found in two files is imported once and counted.
- **New report fields:**
  - `source_format`, `tweet_files`, `posts_in_tweet_files`, `tweet_headers`
  - `community_posts_seen`, `deleted_posts_skipped`, `duplicate_posts_skipped`
  - `archive_entries_rejected`, `media_files_in_archive`, `media_unmatched`
- **Unchanged:** the note format, provenance, thread recovery, and re-import
  behaviour.

`internal/importers/twitter/j25_archive_test.go` checks a synthetic split
archive, both as a folder and as a ZIP:
- every post from three parts and the community post is imported
- the deleted post is not
- the counts match, and the part order is kept
- the media file is imported and none is unmatched
- a reply in part one joins its thread in part zero, which needed part one to
  be read at all
- a header count that differs is reported

The existing fixture and dry-run tests still pass.

## J25-B: bounded memory on untrusted input

- **Streaming.** Post files are decoded one entry at a time
  (`decodeYTDArray`), through a 64 KiB buffer, never read whole.
  - The media index comes from the ZIP's directory.
  - Media are streamed from the ZIP into the asset store.
  - The importer creates no temporary files.
- **Bounds** are package variables, so tests can lower them:

  | bound | value |
  |---|---|
  | entries in the ZIP | 1,000,000 |
  | one decompressed data file | 1 GiB |
  | one decompressed media file | 4 GiB |
  | the `window.YTD` prefix before a JSON array | 4 KiB |

  A file whose recorded size understates it is stopped by the stream limit,
  not only by the recorded size.

`j25_hostile_test.go` checks:
- **An oversized post file** is refused with `errTooLarge`, from a ZIP and from
  a folder. The file is a highly compressible megabyte under a 64 KiB limit:
  the shape of a decompression bomb.
- **The stream limit** stops at the bound.
- **Too many entries** refuse the archive.
- **Unsafe names** are refused and counted. The three tried were an escaping
  `../../` name, an absolute name and a backslash name. Only the safely named
  media is imported, and nothing lands on disk.
- **An oversized media file** is skipped with a warning, without failing the
  import.
- **Neither a folder nor a ZIP:** a plain file is refused clearly.
- **No post files:** a ZIP without them is refused clearly.
- **Odd directory entries** are not reported as refusals (see below).

### Found on the real archive: X's own directory entries

The first ZIP dry run of the new importer refused **9 entries**. The rules the
run applied were then checked against the ZIP's own directory:
- **What they were:** all nine are directory entries whose names end in `//`,
  such as `assets//`, `assets/images//` and
  `assets/images/twemoji/v/latest/svg//`. X writes them into its archives.
- **Why they were refused:** the safety check strips one trailing slash and
  then finds an empty segment.
- **No content lost:** a directory entry holds nothing, and the importer never
  creates directories.

The warning was simply wrong about an ordinary download. Every entry ending in
`/` is now skipped without being counted, and a test covers it. The next run
refused none.

The run also explained the one-file gap in the media count. The ZIP lists
8,737 entries under `data/tweets_media/`, and one of them is that directory's
own entry. That leaves 8,736 media files, which is what the importer counts.

## J25-C: the real archive, measured

### Dry runs

| | downloaded ZIP, new importer | extracted post files, new importer | extracted post files, old importer |
|---|---|---|---|
| post files read | all 4, in part order | all 4 | `tweets.js` only |
| posts in post files | 176,423, equal to `tweet-headers.js` | 176,423 | 55,339 |
| community posts / deleted posts | 1 imported / 6 skipped | (not extracted) | not read |
| posts to import | 176,424 | 176,423 | 55,339 |
| threads recovered | 16,900 | 16,900 | 8,316 |
| media files / unmatched | 8,736 / 0 | (not extracted) | — |
| entries refused | 0 | — | — |
| wall time | 23.4 s | 13.6 s | 5.3 s |
| peak RSS | 237 MB | 230 MB | 344 MB |

The new importer reads **3.2 times as many posts** as the old one did, in
**less** peak memory: it streams each part rather than holding one whole file.
The ZIP run is slower than the folder run because it decompresses as it reads.

### The import, and the re-import

One run of `real_archive_run.sh`, detached, on the archive as downloaded:

| | dry run | import | re-import |
|---|---|---|---|
| wall clock | 23.4 s | **1:14:51** | 39:53.8 |
| peak RSS (`/usr/bin/time`) | 237 MB | **325 MB** | 260 MB |
| peak RSS (sampled every 5 s) | 203 MiB | 318 MiB | 254 MiB |
| posts found | 176,424 | 176,424 | 176,424 |
| notes imported / updated / unchanged | — | 176,424 / 0 / 0 | 0 / 0 / **176,424** |
| media imported / missing / unmatched | — | 8,736 / 0 / 0 | 0 / 0 / 0 |
| tags applied | — | 40,472 | 40,472 |
| attachments linked | — | 8,736 | 8,736 |
| warnings | 0 | 0 | 0 |

- **Everything in the archive is accounted for.** 176,423 posts, exactly what
  `tweet-headers.js` lists, plus the one community post; 6 deleted posts
  skipped; 8,736 media files, all attached to a post and all imported; nothing
  duplicated, nothing unmatched, no entry refused, no warning.
- **The library** is 841,420,800 bytes, from a 3.32 GB archive.
- **The re-import changed nothing.** Every note came back unchanged and the
  library was byte-for-byte the same size afterwards. An interrupted import can
  simply be run again.
- **Memory is bounded and flat.** The peak is about 325 MB for a 3.32 GB
  archive, and the import's own peak is only 88 MB above the dry run's, which
  is the media streaming and the store.
- **Nothing was left in the temp directory** (J22's rule holds here).

### Does writing one document at a time need the batched path?

The note count was sampled every minute through the import:

| notes written | rate | library |
|---|---|---|
| 14,751 → 37,760 | 34.8/s | 203 MB |
| 37,760 → 63,094 | 38.3/s | 331 MB |
| 63,094 → 87,545 | 36.9/s | 456 MB |
| 87,545 → 112,205 | 37.3/s | 581 MB |
| 112,205 → 141,215 | 43.8/s | 715 MB |
| 141,215 → 170,825 | 44.7/s | 827 MB |
| 170,825 → 175,231 | 36.4/s | 843 MB |

**The rate does not degrade as the library grows**: it stays between 35 and 45
notes a second from the first sample to the last, while the library grows from
203 MB to 843 MB. Average 39.2 notes a second.

So J17-C's per-document writes are a **flat cost, not a scaling defect**. That
is the opposite of what J19 and J20 found for the Obsidian inventory, where
cost grew with size. A batched, resumable path would make this import faster,
but nothing here fails or degrades without one, and the re-import shows an
interrupted import can be re-run instead of resumed.

**Recorded for the owner, not decided here:** whether a batched import for the
Twitter, ChatGPT and Claude importers is worth its own item. The measurement
that would justify it is above: 1 h 15 m for 176,424 posts and 3.3 GB of media.

## Documentation and the evidence that pins it

- **CLI usage** now reads `<twitter-archive.zip | extracted-archive-dir>`, in
  `cmd/notriosctl`, `internal/clispec/commands.json` (regenerated into
  `docs/cli.md`) and `API_SPEC.md`.
- **`docs/import-export.md`** tells a user to give the importer the ZIP as it
  downloaded, and describes the part files, community and deleted posts, the
  header check and the refusals. `docs/cli.md`'s `import twitter` section says
  the same.
- **The docs example stays executed.** The Twitter/X example in
  `import-export.md` was an *executed* example, run against a seeded extracted
  folder. It is still executed, now against the same fixture **zipped** (a new
  `import.twitter_zip` substitution), so the documentation's own example proves
  that a downloaded ZIP imports.
- **Its registry entry moved** with the renamed section
  (`twitter-x-extracted-archive` to `twitter-x-archive`).
- **Docs site.** `docs-site/layouts/_markup/render-heading.html` maps the new
  anchor, and keeps the old one as an alias so existing links still land.
- **Pinned evidence brought forward,** each diff checked:
  - the G18a inventory: two doc hashes and the renamed section
  - G18f's `docs/cli.md` hash
  - J13's example classification: the renamed example, and the settable key
    count 53 to 54, which records J22's `data.temp_dir`

## How the measurements were taken

- **Isolation:** every run pointed HOME, the XDG roots and TMPDIR at its own
  directory on `/media/renes/HD2`, never RAM-backed `/tmp`.
- **The runner:** `real_archive_run.sh` runs the dry run, the import and the
  re-import in turn. It times each with `/usr/bin/time -v`, and samples RSS and
  MemAvailable every 5 s, with a guard that stops the run under 8 GiB
  available.
- **Detached:** it was started with `setsid`/`nohup`, as J7-C's long drill was.
- **Other work during the import:** only light checks ran while it did: Python
  evidence validators, and the `docaudit` and importer test packages. The Go
  test suites that build binaries, and the docs-site build, were deferred until
  after it.
