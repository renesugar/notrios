#!/usr/bin/env bash
# Build the internal Ubuntu package.
#
# The layout comes from scripts/lifecycle.py, staged through DESTDIR, so the
# package and `make install` cannot disagree about where anything goes. H6a
# recommended exactly that: two definitions of an install contract drift, and
# the one that drifts silently is the packaged one, because nobody runs it
# during development.
#
# It builds with dpkg-deb rather than through GoReleaser's nFPM integration.
# H6a selected GoReleaser as reasonable and this is a deliberate narrowing, for
# reasons H6a itself measured: every gap it found -- no declared dependencies,
# no maintainer scripts, no copyright, source file modes -- is something nFPM
# does not do and dpkg tooling does. dpkg-shlibdeps computes the dependency set
# from the ELF and the system shlibs database; nFPM has no equivalent. dpkg-deb
# is present on every Ubuntu build host, so this adds no dependency at all,
# which is what H6a's "smallest maintainable toolchain" asked for. Moving back
# to nFPM later is a configuration change, not a rewrite.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${1:-"$ROOT/dist/deb"}
WORK=$(mktemp -d -t notrios-deb-XXXXXX)
trap 'rm -rf "$WORK"' EXIT

VERSION=$(sed -n 's/.*Version = "\([^"]*\)".*/\1/p' "$ROOT/internal/version/version.go")
REVISION=${DEB_REVISION:-1}
ARCH=$(dpkg --print-architecture)
STAGE="$WORK/stage"

for tool in dpkg-deb dpkg-shlibdeps dpkg-architecture; do
  command -v "$tool" >/dev/null || { echo "$tool is required to build the package" >&2; exit 1; }
done

# 1. Build the binaries the package will ship, with packaging flags.
#
#    Not reused from bin/: those are whatever the developer last built. Three
#    flags matter here and none are the default. -buildmode=pie answers
#    lintian's hardening-no-pie; -ldflags="-s -w" answers unstripped-binary,
#    without running strip over a Go binary and disturbing its DWARF; -trimpath
#    removes the build machine's directory names from the shipped artifact,
#    which is both a reproducibility and a privacy property.
echo "building $VERSION-$REVISION ($ARCH)"
SRC="$WORK/src"
mkdir -p "$SRC/bin"
ln -s "$ROOT/web" "$SRC/web"
ln -s "$ROOT/docs" "$SRC/docs"
ln -s "$ROOT/internal" "$SRC/internal"
ln -s "$ROOT/assets" "$SRC/assets"

PACKAGE_FLAGS=(-trimpath -buildmode=pie -ldflags "-s -w")
( cd "$ROOT" && go build "${PACKAGE_FLAGS[@]}" -o "$SRC/bin/notriosd" ./cmd/notriosd )
( cd "$ROOT" && go build "${PACKAGE_FLAGS[@]}" -o "$SRC/bin/notriosctl" ./cmd/notriosctl )
# The desktop binary needs GTK and WebKit headers. A build host without them can
# still produce a service-and-CLI package rather than failing outright, and the
# manifest records what was actually included.
if ( cd "$ROOT" && go build "${PACKAGE_FLAGS[@]}" -tags "gui desktop production webkit2_41" \
       -o "$SRC/bin/notrios" ./cmd/notrios 2>"$WORK/gui.log" ); then
  echo "  including the desktop GUI"
else
  echo "  no desktop GUI: $(tail -1 "$WORK/gui.log")" >&2
fi

# 2. Stage exactly what `make install` would write, with the packaged prefix.
#    DESTDIR never touches a user root, so staging creates no mutable data --
#    the property H6 requires and H5 already implements.
env prefix=/usr DESTDIR="$STAGE" NOTRIOS_LIFECYCLE_SOURCE="$SRC" \
    python3 "$ROOT/scripts/lifecycle.py" install >/dev/null

# The install manifest is deliberately not shipped. dpkg owns the file list for
# a packaged install and records its own md5sums; a second ownership record in
# the same tree is the drift this script exists to avoid.
rm -f "$STAGE/usr/share/notrios/MANIFEST.json"

# 3. Debian metadata that lintian requires and nFPM did not produce.
mkdir -p "$STAGE/DEBIAN" "$STAGE/usr/share/doc/notrios"

install -m 0644 "$ROOT/LICENSE" "$STAGE/usr/share/doc/notrios/LICENSE"
cat > "$STAGE/usr/share/doc/notrios/copyright" <<'COPYRIGHT'
Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: notrios
Source: https://github.com/renesugar/notrios

Files: *
Copyright: 2026 Rene Sugar
License: Apache-2.0
 Licensed under the Apache License, Version 2.0 (the "License"); you may not
 use this file except in compliance with the License. You may obtain a copy of
 the License at
 .
     http://www.apache.org/licenses/LICENSE-2.0
 .
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 License for the specific language governing permissions and limitations under
 the License.
 .
 On Debian systems the complete text of the Apache License version 2.0 can be
 found in /usr/share/common-licenses/Apache-2.0, and a copy is also installed at
 /usr/share/doc/notrios/LICENSE.
COPYRIGHT
chmod 0644 "$STAGE/usr/share/doc/notrios/copyright"

printf 'notrios (%s-%s) unstable; urgency=low\n\n  * Internal prerelease build. Not a public release.\n\n -- Rene Sugar <rene.sugar@gmail.com>  %s\n' \
  "$VERSION" "$REVISION" "$(date -R)" > "$WORK/changelog"
gzip -9n -c "$WORK/changelog" > "$STAGE/usr/share/doc/notrios/changelog.Debian.gz"
chmod 0644 "$STAGE/usr/share/doc/notrios/changelog.Debian.gz"

# 4. A user service that is installed and not enabled.
#    It binds the compiled loopback default and widens no network access; a user
#    turns it on with `systemctl --user enable --now notrios`.
mkdir -p "$STAGE/usr/lib/systemd/user"
cat > "$STAGE/usr/lib/systemd/user/notrios.service" <<'UNIT'
[Unit]
Description=Notrios local note service
Documentation=file:///usr/share/notrios/docs/service.md

[Service]
Type=simple
ExecStart=/usr/bin/notriosd
Restart=on-failure

[Install]
WantedBy=default.target
UNIT
chmod 0644 "$STAGE/usr/lib/systemd/user/notrios.service"

# 5. Dependencies computed from the binaries rather than guessed.
mkdir -p "$WORK/debian"
printf 'Source: notrios\nPackage: notrios\nArchitecture: %s\n' "$ARCH" > "$WORK/debian/control"
BINARIES=()
for name in notriosd notriosctl notrios; do
  test -f "$STAGE/usr/bin/$name" && BINARIES+=("$STAGE/usr/bin/$name")
done
DEPENDS=$( cd "$WORK" && dpkg-shlibdeps -O --ignore-missing-info "${BINARIES[@]}" 2>/dev/null \
           | sed 's/^shlibs:Depends=//' )
test -n "$DEPENDS" || { echo "dpkg-shlibdeps produced no dependencies; refusing to ship an underdeclared package" >&2; exit 1; }

INSTALLED_SIZE=$(du -ks "$STAGE" | cut -f1)
cat > "$STAGE/DEBIAN/control" <<CONTROL
Package: notrios
Version: $VERSION-$REVISION
Section: utils
Priority: optional
Architecture: $ARCH
Depends: $DEPENDS
Maintainer: Rene Sugar <rene.sugar@gmail.com>
Homepage: https://github.com/renesugar/notrios
Installed-Size: $INSTALLED_SIZE
Description: local-first note application
 Notrios keeps notes in a local SQLite library that you own, with a desktop
 interface, a headless service exposing REST and MCP, and a command-line tool
 for import, export, maintenance and synchronisation.
 .
 Nothing is uploaded anywhere. The service listens on loopback only and is not
 started until you enable it.
CONTROL
chmod 0644 "$STAGE/DEBIAN/control"

# 6. Maintainer scripts. Without these the desktop entry is installed and never
#    takes effect: nothing tells the desktop environment that a new handler for
#    notrios:// exists.
cat > "$STAGE/DEBIAN/postinst" <<'POSTINST'
#!/bin/sh
set -e
if [ "$1" = "configure" ]; then
    # Both are best-effort: a headless machine has neither, and failing an
    # install because a desktop database is missing would be absurd.
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database -q /usr/share/applications || true
    fi
    if command -v systemctl >/dev/null 2>&1; then
        systemctl daemon-reload >/dev/null 2>&1 || true
    fi
fi
exit 0
POSTINST

cat > "$STAGE/DEBIAN/postrm" <<'POSTRM'
#!/bin/sh
set -e
# Removal never touches a user's notes, configuration, profiles, keys, state or
# cache. Deleting those is `notriosctl`/`make purge`, which asks first.
if [ "$1" = "remove" ] || [ "$1" = "purge" ]; then
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database -q /usr/share/applications || true
    fi
fi
exit 0
POSTRM
chmod 0755 "$STAGE/DEBIAN/postinst" "$STAGE/DEBIAN/postrm"

# 7. Normalise modes. nFPM shipped the checkout's 0664 on every documentation
#    file, which lintian flags as non-standard-file-perm.
find "$STAGE/usr" -type d -exec chmod 0755 {} +
find "$STAGE/usr" -type f -exec chmod 0644 {} +
for name in notriosd notriosctl notrios; do
  test -f "$STAGE/usr/bin/$name" && chmod 0755 "$STAGE/usr/bin/$name"
done

# 8. Check what optional artifacts actually made it in.
#
#    The icon theme is optional in lifecycle.py, which is right for a checkout
#    that has not generated it -- and wrong to trust here. The first packaged
#    build shipped no icons at all: the staging tree symlinks the directories it
#    copies from, assets/ was not among them, and an optional artifact that is
#    missing is skipped without complaint. Silence is exactly what an optional
#    artifact gives you, so the package build asserts instead of assuming.
if test -d "$ROOT/assets/icons"; then
  ICON_COUNT=$(find "$STAGE/usr/share/icons/hicolor" -name 'notrios.png' 2>/dev/null | wc -l)
  if test "$ICON_COUNT" -eq 0; then
    echo "the checkout has assets/icons but the package staged none; refusing to ship an iconless package" >&2
    exit 1
  fi
  echo "  staged $ICON_COUNT icon sizes"
fi

mkdir -p "$OUT"
PACKAGE="$OUT/notrios_${VERSION}-${REVISION}_${ARCH}.deb"
dpkg-deb --root-owner-group --build "$STAGE" "$PACKAGE" >/dev/null
echo "built $PACKAGE"
sha256sum "$PACKAGE"
