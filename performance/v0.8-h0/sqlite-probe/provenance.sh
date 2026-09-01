#!/bin/sh
set -eu
archive=${1:-sqlite-amalgamation-3530400.zip}
url=https://www.sqlite.org/2026/${archive}
expected=628a44cfe82c66aed1ccbbe85a562d2e33ebe64b3288981ed76285612227934e
if [ ! -f "$archive" ]; then curl --fail --location --output "$archive" "$url"; fi
if command -v sha3sum >/dev/null 2>&1; then actual=$(sha3sum -a 256 "$archive" | awk '{print $1}'); elif command -v openssl >/dev/null 2>&1; then actual=$(openssl dgst -sha3-256 "$archive" | awk '{print $NF}'); else echo "sha3sum or openssl is required" >&2; exit 1; fi
[ "$actual" = "$expected" ] || { echo "SHA3 mismatch: $actual" >&2; exit 1; }
echo "archive=$archive"
echo "sha3-256=$actual"
unzip -l "$archive" | head -n 5
