#!/usr/bin/env python3
"""Validate the v0.8 H4a port record against the working tree.

The record is only worth keeping if it stays true. This asserts the counts it
claims for every page, and -- the part that actually matters -- that the two
default addresses have not drifted back together, which is the whole point of
H4a and the one failure no page-by-page reading would notice.
"""
import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
HERE = pathlib.Path(__file__).resolve().parent

DEVELOPMENT = "127.0.0.1:8099"
INSTALLED = "127.0.0.1:8080"


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    record = json.loads((HERE / "PORT_MENTIONS.json").read_text(encoding="utf-8"))
    require(record["schema"] == "notrios.h4a.port-mentions.v1", "wrong record schema")
    require(record["development_address"] == DEVELOPMENT, "development address drifted")
    require(record["installed_address"] == INSTALLED, "installed address drifted")

    # The two addresses must differ, and each must live where it belongs.
    example = (ROOT / "config/config.example.yaml").read_text(encoding="utf-8")
    require(f'listen_addr: "{DEVELOPMENT}"' in example,
            "the checkout example config no longer listens on the development address")
    require(f'public_base_url: "http://{DEVELOPMENT}"' in example,
            "the checkout example config advertises a base URL that is not its own address")
    # Only the settings matter here. The comment beside them names the installed
    # address deliberately, to say what the checkout is being kept apart from.
    example_settings = [
        line for line in example.splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    require(all(INSTALLED not in line for line in example_settings),
            "a setting in the checkout example config names the installed address, so the two would collide")

    compiled = (ROOT / "internal/config/config.go").read_text(encoding="utf-8")
    require(f'ListenAddr:    "{INSTALLED}"' in compiled,
            "the compiled default is no longer the installed address")

    # `make serve` must not override the example config back onto 8080.
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    serve = [line for line in makefile.splitlines() if line.strip().startswith("go run ./cmd/notriosd")]
    require(serve, "the serve target no longer runs notriosd")
    require(all(INSTALLED not in line for line in serve),
            f"`make serve` binds {INSTALLED}, the installed instance's address")

    # The bind-failure advice must not recommend either default.
    service = (ROOT / "internal/service/service.go").read_text(encoding="utf-8")
    require(f"-addr {DEVELOPMENT}" not in service,
            "the bind-failure advice recommends the development address")

    # Every page still holds the mentions the record claims.
    for page in record["pages"]:
        path = ROOT / page["path"]
        require(path.exists(), f"recorded page is missing: {page['path']}")
        body = path.read_text(encoding="utf-8")
        installed = len(re.findall(re.escape("8080"), body))
        development = len(re.findall(re.escape("8099"), body))
        require(installed == page["installed_mentions"],
                f"{page['path']}: {installed} mentions of 8080, record says {page['installed_mentions']}")
        require(development == page["development_mentions"],
                f"{page['path']}: {development} mentions of 8099, record says {page['development_mentions']}")

    pages = len(record["pages"])
    installed_total = sum(p["installed_mentions"] for p in record["pages"])
    development_total = sum(p["development_mentions"] for p in record["pages"])
    print(
        f"H4a evidence valid: {pages} pages, {installed_total} installed-address mentions, "
        f"{development_total} development-address mentions, both defaults distinct."
    )


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"H4a evidence invalid: {error}", file=sys.stderr)
        sys.exit(1)
