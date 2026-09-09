#!/bin/sh
set -eu
cd "$(dirname "$0")"
out="${TMPDIR:-/tmp}/notrios-h0-abi-$$"
trap 'rm -rf "$out"' EXIT
mkdir -p "$out"
go build -buildmode=c-shared -o "$out/libnotrios_abi.so" .
test -s "$out/libnotrios_abi.h"
go build -buildmode=c-archive -o "$out/libnotrios_abi.a" .
test -s "$out/libnotrios_abi.h"
cp "$out/libnotrios_abi.h" ./generated_notrios_abi.h
printf '%s\n' notrios_abi_version notrios_capabilities notrios_instance_open notrios_instance_close notrios_call_start notrios_call_poll notrios_call_cancel notrios_buffer_release notrios_event_poll notrios_stream_open notrios_stream_read notrios_stream_close | sort > "$out/expected"
nm -D --defined-only "$out/libnotrios_abi.so" | awk '$3 ~ /^notrios_/ {print $3}' | sort > "$out/actual"
cmp -s "$out/expected" "$out/actual"
cc -std=c11 -Wall -Wextra -Werror -I. host.c -L"$out" -lnotrios_abi -Wl,-rpath,"$out" -pthread -o "$out/host"
"$out/host"
echo "abi-probe: c-shared/c-archive/header/symbol/host checks PASS"
