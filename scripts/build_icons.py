#!/usr/bin/env python3
"""Generate the application icons from the source artwork.

The source is a dark navy rounded tile on an opaque near-white canvas. That
canvas is the whole problem: it renders as a white box on a dark desktop theme
or a dark browser tab. The artwork itself needs no recolouring, because the tile
already carries its own dark background with light content on it.

Three things are produced.

**Transparent icons** for the desktop and the light-mode favicon: the canvas is
removed and the tile floats.

**An SVG favicon** derived from the vector master beside the raster one, when
that master is present. It needs no light/dark pair, because an SVG can carry
its own prefers-color-scheme rule; it gets the same canvas removal and the same
rim, in vector terms.

**A dark-mode favicon** with a faint rim. The brand navy #132965 has 13.7:1
contrast on white and 1.03:1 on a dark browser tab strip -- the tile silhouette
disappears entirely there, leaving a white N floating in nothing. The rim
restores the outline without touching the brand colour, which is not this
script's decision to change.

Alpha is reconstructed by un-blending against the known canvas colour rather
than by thresholding. The tile edge is anti-aliased against near-white, so a
threshold leaves those blended pixels opaque and pale, and the icon gets a
bright fringe on exactly the dark backgrounds this exists to fix.

The output is committed rather than generated at build time. `web/public` must
exist before `npm run build` or the shipped interface has no favicon and nothing
warns; `assets/icons` must exist before `make install` or `make deb` or the
package quietly ships without icons -- and the guard in build_deb.sh cannot catch
that one, because it only fires when the directory is present and staging missed
it. Regenerate with `make icons` after changing assets/notrios.png or
assets/notrios.svg.
"""

from __future__ import annotations

import os
import re
import sys

from PIL import Image, ImageDraw, ImageFilter

SOURCE = os.environ.get("NOTRIOS_ICON_SOURCE", "assets/notrios.png")
SVG_SOURCE = os.environ.get("NOTRIOS_ICON_SVG", "assets/notrios.svg")
CANVAS = (249, 249, 249)      # the near-white surround in the source
BRAND = (19, 41, 101)         # #132965, the tile navy: never altered
RIM = (150, 170, 225)         # a light rim, only for the dark-mode favicon
# The near-white canvas as the vector master spells it. The raster constant
# above is one colour; a trace approximates it, so the SVG may say any of these.
CANVAS_FILLS = {"#F9F9F9", "#F8F9F9", "#FAFAFA", "#FDFDFD"}

# Freedesktop hicolor sizes. 16 through 256 is what a Linux desktop actually
# looks for; the favicon package's 96/180/192/512 do not cover them.
DESKTOP_SIZES = (16, 24, 32, 48, 64, 128, 256)


def reconstruct_alpha(image: Image.Image, canvas=CANVAS) -> Image.Image:
    """Turn the surrounding canvas into transparency, edges included.

    The background is the region **reachable from the border**, not "every pixel
    close to the canvas colour". Those are different, and the difference matters
    here: the N is #fefefe on a #f9f9f9 canvas, five levels apart, so a colour
    test makes the white artwork inside the tile transparent and the icon shows
    the desktop through its own lettering. That is what the first version of
    this function did, and it looked fine in every numeric check -- no stray
    fringe, correct corners, full alpha range -- until the icon was composited
    on a dark background and the N was dark.

    So: flood fill from the border to find the true outside, then un-blend only
    the anti-aliased ring at its edge. A pixel on that ring is
    `a*F + (1-a)*canvas`; solving for `a` and recovering F keeps the tile colour
    at partial alpha instead of leaving a pale mix that fringes on dark
    backgrounds.
    """
    image = image.convert("RGBA")
    width, height = image.size

    # Distance from the canvas colour, as an 8-bit image flood fill can walk.
    distance = Image.new("L", (width, height))
    source = image.load()
    dist_pixels = distance.load()
    for y in range(height):
        for x in range(width):
            r, g, b, _ = source[x, y]
            dist_pixels[x, y] = min(255, max(abs(r - canvas[0]),
                                             abs(g - canvas[1]),
                                             abs(b - canvas[2])))

    # The outside: contiguous canvas reached from the border. The N is enclosed
    # by the tile and is never reached, which is the whole point.
    filled = distance.copy()
    for seed in ((0, 0), (width - 1, 0), (0, height - 1), (width - 1, height - 1)):
        ImageDraw.floodfill(filled, seed, 255, thresh=14)
    filled_pixels = filled.load()

    outside = Image.new("L", (width, height), 0)
    outside_pixels = outside.load()
    for y in range(height):
        for x in range(width):
            if filled_pixels[x, y] == 255 and dist_pixels[x, y] < 255:
                outside_pixels[x, y] = 255

    # The anti-aliased ring is the few pixels just inside the outside region.
    ring = outside.filter(ImageFilter.MaxFilter(7))
    ring_pixels = ring.load()

    result = image.copy()
    out = result.load()
    for y in range(height):
        for x in range(width):
            r, g, b, _ = source[x, y]
            if outside_pixels[x, y]:
                out[x, y] = (0, 0, 0, 0)
                continue
            if not ring_pixels[x, y]:
                out[x, y] = (r, g, b, 255)
                continue
            alpha = min(1.0, dist_pixels[x, y] / 90.0)
            if alpha <= 0.02:
                out[x, y] = (0, 0, 0, 0)
            elif alpha >= 0.98:
                out[x, y] = (r, g, b, 255)
            else:
                unblended = tuple(
                    min(255, max(0, int((channel - canvas[i] * (1 - alpha)) / alpha)))
                    for i, channel in enumerate((r, g, b))
                )
                out[x, y] = (*unblended, int(alpha * 255))
    return result


def trim(image: Image.Image) -> Image.Image:
    """Crop to the artwork. The source pads the tile with a wide margin, which
    would otherwise shrink the tile inside every generated size."""
    box = image.getbbox()
    return image.crop(box) if box else image


def add_rim(image: Image.Image) -> Image.Image:
    """Trace the silhouette in a light colour, for the dark-mode favicon."""
    alpha = image.getchannel("A")
    grown = alpha.filter(ImageFilter.MaxFilter(9))
    rim_only = Image.new("RGBA", image.size, (*RIM, 0))
    rim_only.putalpha(grown)
    return Image.alpha_composite(rim_only, image)


def build_svg_favicon(source=SVG_SOURCE, target="web/public/notrios.svg") -> bool:
    """Derive the favicon SVG from the master, the way the PNGs are derived.

    Two things have to happen to the master before it is a favicon, and both are
    the vector form of what this script already does to the raster.

    The canvas goes. The master is a rounded navy tile on a near-white canvas,
    and that canvas is the whole problem this file exists to solve -- it renders
    as a white box on a dark browser tab. In the vector it is one path: a full
    square whose hole is the tile outline. So the outline becomes a clip and the
    canvas path is dropped, which is the same operation reconstruct_alpha
    performs on pixels.

    And a rim is added for dark mode, from that same outline, so it follows the
    rounded corners rather than boxing them. Unlike the PNGs this needs no
    second file: an SVG carries its own prefers-color-scheme rule.

    Returning False rather than raising when there is no master keeps `make
    icons` working for anyone who has the PNG artwork and not the vector.
    """
    if not os.path.isfile(source):
        print(f"no SVG master at {source}; skipping the SVG favicon", file=sys.stderr)
        return False

    master = open(source, encoding="utf-8").read()
    paths = re.findall(r'<path d="([^"]*)" fill="(#[0-9A-Fa-f]{6})"[^/]*/>', master)
    canvas = next(((d, fill) for d, fill in paths if d.count("Z") == 2 and fill.upper() in CANVAS_FILLS), None)
    if canvas is None:
        print(f"{source} has no canvas path to clip away; skipping the SVG favicon", file=sys.stderr)
        return False
    canvas_d, canvas_fill = canvas
    tile = "M" + canvas_d.split("Z")[1].strip().lstrip("M") + " Z"

    element = re.search(r'<path d="' + re.escape(canvas_d) + r'" fill="' + canvas_fill + r'"[^/]*/>\n?', master)
    body = master[:element.start()] + master[element.end():]

    head = re.search(r'<svg[^>]*>', body)
    opening = head.group(0)
    if "viewBox" not in opening:
        body = body.replace(opening, opening[:-1] + ' viewBox="0 0 1254 1254">', 1)
        opening = re.search(r'<svg[^>]*>', body).group(0)

    brand = "#%02X%02X%02X" % BRAND
    rim = "#%02X%02X%02X" % RIM
    preamble = (
        "\n<!-- Generated by scripts/build_icons.py from " + source + ". Do not edit:\n"
        "     regenerate with `make icons`. The canvas is clipped away because it\n"
        "     renders as a white box on a dark browser tab, and the rim below is\n"
        "     the tile outline, so it follows the rounded corners. -->\n"
        "<style>\n"
        "  /* " + brand + " has 13.7:1 contrast on a white page and 1.03:1 on a dark\n"
        "     browser tab strip, where the silhouette disappears and the mark reads\n"
        "     as a white N floating in nothing. */\n"
        "  .tab-rim { fill: none; stroke: none; stroke-width: 34; }\n"
        "  @media (prefers-color-scheme: dark) { .tab-rim { stroke: " + rim + "; } }\n"
        "</style>\n"
        '<defs><clipPath id="tile"><path d="' + tile + '"/></clipPath></defs>\n'
        '<g clip-path="url(#tile)">'
    )
    body = body.replace(opening, opening + preamble, 1)
    body = body.replace("</svg>", '</g>\n<path class="tab-rim" d="' + tile + '"/>\n</svg>')

    os.makedirs(os.path.dirname(target), exist_ok=True)
    with open(target, "w", encoding="utf-8") as handle:
        handle.write(body)
    print(f"wrote {target} from {source}")
    return True


def main() -> int:
    if not os.path.isfile(SOURCE):
        print(f"no icon source at {SOURCE}", file=sys.stderr)
        return 1

    print(f"reading {SOURCE}")
    transparent = trim(reconstruct_alpha(Image.open(SOURCE)))

    # Desktop: hicolor sizes, transparent. No dark variant exists here -- the
    # icon theme specification has no reliable light/dark mechanism for
    # application icons, so one file has to work on both.
    for size in DESKTOP_SIZES:
        target = f"assets/icons/{size}x{size}/notrios.png"
        os.makedirs(os.path.dirname(target), exist_ok=True)
        transparent.resize((size, size), Image.LANCZOS).save(target)
    print(f"wrote {len(DESKTOP_SIZES)} desktop sizes to assets/icons/")

    # Web: a light-mode icon and a rimmed dark-mode one. Browsers pick between
    # them with prefers-color-scheme, which is the one surface where a real
    # dark variant is possible.
    os.makedirs("web/public", exist_ok=True)
    for size in (32, 180, 192, 512):
        transparent.resize((size, size), Image.LANCZOS).save(f"web/public/notrios-{size}.png")
    dark = add_rim(transparent)
    for size in (32, 192):
        dark.resize((size, size), Image.LANCZOS).save(f"web/public/notrios-dark-{size}.png")
    transparent.resize((32, 32), Image.LANCZOS).save("web/public/favicon-32.png")

    # A multi-size .ico for the browsers and pinned-tab contexts that still ask
    # for one. Generated here rather than taken from the supplied favicon
    # package, whose PNGs bake in the near-white canvas this script removes:
    # mixing the two would ship a transparent icon in one slot and a white box
    # in the next.
    transparent.resize((256, 256), Image.LANCZOS).save(
        "web/public/favicon.ico", sizes=[(16, 16), (32, 32), (48, 48), (64, 64)])

    # Maskable icons are a different shape of problem. The platform crops them
    # to whatever silhouette it likes -- circle, squircle, rounded square -- so
    # they must bleed to the edges. A transparent icon masked to a circle leaves
    # the brand colour floating in a cropped void, and the supplied package's
    # maskable entries have the near-white canvas at the edges, which masks to a
    # white ring. So the tile colour fills the square and the artwork sits
    # inside the safe zone, scaled to 80% per the specification's guidance.
    for size in (192, 512):
        maskable = Image.new("RGBA", (size, size), (*BRAND, 255))
        inner = int(size * 0.8)
        art = transparent.resize((inner, inner), Image.LANCZOS)
        offset = (size - inner) // 2
        maskable.paste(art, (offset, offset), art)
        maskable.save(f"web/public/notrios-maskable-{size}.png")

    print("wrote web/public/ light, dark, ico and maskable icons")

    build_svg_favicon()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
