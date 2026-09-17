#!/usr/bin/env bash
# Confirm a purge removed what it said it would (v1.0 J11).
#
#   notriosctl paths --report --paths > ~/notrios-before-purge.txt   # before
#   notriosctl purge --confirm                                       # the purge
#   bash scripts/check_purged.sh ~/notrios-before-purge.txt          # after
#
# It reads a manifest taken *before* the purge, because everything Notrios
# installed is gone afterwards -- including anything that could read a manifest,
# and including the manifest itself if it was written inside the library. Keep
# the copy somewhere the purge does not reach.
#
# Nothing but a shell is required, for the same reason: at the moment this runs,
# a Go binary, a Python interpreter or a JSON parser belonging to Notrios is
# exactly what is not there. Each line is `owned<TAB>path` or `external<TAB>path`.
#
# Two failures, not one:
#   - an owned path that still exists, which means the purge was incomplete;
#   - an external path that is gone, which means a purge deleted a library it
#     was supposed to keep. That is worse, and it is reported separately.
set -uo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

manifest=${1:-}
if [[ -z "$manifest" || ! -f "$manifest" ]]; then
  echo "usage: bash scripts/check_purged.sh <manifest-from-notriosctl-paths---report---paths>" >&2
  echo "the manifest must be a copy taken before the purge, kept outside the library" >&2
  exit 2
fi

owned_total=0 owned_left=0 external_total=0 external_missing=0 malformed=0
left=() missing=()

while IFS=$'\t' read -r state path || [[ -n "${state:-}" ]]; do
  [[ -z "${state:-}" ]] && continue
  case "$state" in
    owned)
      owned_total=$((owned_total + 1))
      # -e is false for a dangling symlink, which is still something left
      # behind, so test the link itself as well.
      if [[ -e "$path" || -L "$path" ]]; then
        owned_left=$((owned_left + 1))
        left+=("$path")
      fi
      ;;
    external)
      external_total=$((external_total + 1))
      if [[ ! -e "$path" && ! -L "$path" ]]; then
        external_missing=$((external_missing + 1))
        missing+=("$path")
      fi
      ;;
    *)
      malformed=$((malformed + 1))
      ;;
  esac
done < "$manifest"

if (( malformed > 0 )); then
  echo "$manifest has $malformed line(s) that are neither owned nor external; is it a --paths manifest?" >&2
  exit 2
fi
if (( owned_total == 0 && external_total == 0 )); then
  echo "$manifest lists nothing, so it proves nothing" >&2
  exit 2
fi

status=0
if (( owned_left > 0 )); then
  echo "the purge left $owned_left of $owned_total path(s) behind:" >&2
  for path in "${left[@]}"; do
    echo "  $path" >&2
  done
  status=1
fi
if (( external_missing > 0 )); then
  echo "$external_missing path(s) a purge keeps are gone; a library outside the roots was deleted:" >&2
  for path in "${missing[@]}"; do
    echo "  $path" >&2
  done
  status=1
fi

if (( status == 0 )); then
  echo "purge verified: $owned_total owned path(s) gone, $external_total external path(s) still present"
fi
exit "$status"
