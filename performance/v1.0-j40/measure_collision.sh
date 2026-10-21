#!/usr/bin/env bash
# v1.0 J40-C: what the ambiguous-links report costs on J32's collision shape.
set -euo pipefail
W=/run/media/renes/HD2/j40m
CORPUS=/run/media/renes/HD2/notrios-j39/corpora/collision
rm -rf "$W"; mkdir -p "$W"/home/{config,data,cache,state} "$W"/tmp "$W"/bin "$W"/out
cd /home/renes/projects/notrios
go build -o "$W/bin/notriosctl" ./cmd/notriosctl
iso() { env -u DBUS_SESSION_BUS_ADDRESS HOME="$W/home" XDG_CONFIG_HOME="$W/home/config" XDG_DATA_HOME="$W/home/data" \
  XDG_CACHE_HOME="$W/home/cache" XDG_STATE_HOME="$W/home/state" TMPDIR="$W/tmp" "$@"; }
ROOTS=(--db "$W/lib/notes.sqlite" --asset-store "$W/lib/assets")
mkdir -p "$W/lib"
iso /usr/bin/time -f "%M %e" -o "$W/out/import.time" "$W/bin/notriosctl" import obsidian "${ROOTS[@]}" "$CORPUS" > "$W/out/import.json"
echo "import: $(cat "$W/out/import.time")"
for run in 1 2 3 4 5; do
  for cmd in links lint graph; do
    case $cmd in
      links) args=(links report) ;;
      lint)  args=(lint --quiet) ;;
      graph) args=(graph report) ;;
    esac
    iso /usr/bin/time -f "%M %e" -o "$W/out/$cmd-$run.time" "$W/bin/notriosctl" "${args[@]}" "${ROOTS[@]}" > "$W/out/$cmd-$run.json" || true
    echo "$cmd run $run: $(cat "$W/out/$cmd-$run.time")"
  done
done
iso /usr/bin/time -f "%M %e" -o "$W/out/note.time" "$W/bin/notriosctl" links report --write-note "${ROOTS[@]}" > "$W/out/note.json"
echo "write-note: $(cat "$W/out/note.time")"
iso "$W/bin/notriosctl" notes show --document doc_link_report "${ROOTS[@]}" > "$W/out/note-body.txt"
wc -c "$W/out/note-body.txt"
