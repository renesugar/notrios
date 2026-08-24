# v0.7 G16 Sync Center evidence

This directory records generated, content-free validation for G16. It contains
no private notes, imported corpus, secret key, credential, password, raw job
parameter, filesystem topology, or backup artifact.

## Two-process product flow

`TestSyncUITwoProcessRecoveryFlow` builds `notriosctl` and `notriosd`, starts two
real loopback daemon processes holding adopted replicas of one generated
library, and drives the same local HTTP routes React calls. It validates:

1. a short-lived invitation and explicit REST pairing;
2. separate permission to receive a complete catch-up snapshot;
3. bidirectional encrypted operation exchange;
4. visible base/current/other overlap and an explicit two-parent resolution;
5. lazy attachment metadata, request publication, host serving, explicit retry,
   and exact materialized bytes;
6. NPB1 creation, wrong-password refusal, correct-password verification, and
   `applied=false`; and
7. authenticated physical catch-up into private staging without installation.

Validated 2026-08-24:

```text
go test ./cmd/notriosctl -run TestSyncUITwoProcessRecoveryFlow -count=1 -v
--- PASS: TestSyncUITwoProcessRecoveryFlow (18.03s)
```

Loopback tests require the normal host environment because the repository
sandbox intentionally blocks socket creation.

## Rendered browser flow

The Browser plugin was not installed. Following the frontend testing skill,
regular Playwright 1.63 used installed Google Chrome against a fresh generated
profile served by the real daemon and built `web/dist`.

| Viewport | Dialog bounds | Result |
|---|---:|---|
| 1440×960 | x=160, y=70, 1120×820 | pass |
| 390×844 | x=0, y=0, 390×844 | pass |

The sweep opens Sync Center, exercises setup and backup/recovery navigation,
proves a browser cannot see the native Wails directory chooser, verifies
password show/hide, checks every visible mobile control reaches the 44px target
(allowing subpixel rounding), closes with Escape, and reports zero console or
page errors. The sweep found and fixed an undersized mobile close button and
file input. Screenshots contain only generated empty-profile/Help UI and fixture
text and are handed off outside the repository with the release ZIP.
