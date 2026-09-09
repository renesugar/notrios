# G20 hardening analysis context

Analysis ID: `hardening_final`

The source is completed Codex Security scan
`a8529c1e-bc60-472a-ae36-de6067741fdd`, stored locally at
`/tmp/codex-security-scans-0oCjwp/notrios/a3cea5a39f9c0bfe20e80c7615b503ac09967cce_20260831T030931Z_ir4u34zq/`.

- Sealed manifest SHA-256: `061c5d52f17208dcd1ec2ebff9f0c2b0abb8048e9c47f5064fe265f71861f661`
- Findings SHA-256: `0e84855ae4f18d528d23a2965d88643351a0748ecfa363b5d8f1b78136eaf7ba`
- Coverage SHA-256: `f9ac37f7b978fd43e42234cf7de278ca06b9da10c52f64049e2249663ca9b840`
- Target revision: `a3cea5a39f9c0bfe20e80c7615b503ac09967cce`
- Snapshot: `codex-security-snapshot/v1:sha256:ca4e5ab0f03fd2ae4edac30421288c61c159b04f856021f589b5b7baa548a311`
- Source drift: present. G20's working tree contains candidate tactical
  remediations and release changes after the scanned baseline.

The six canonical findings are two high-confidence/high-severity network and
TLS lifecycle failures, plus four high-confidence/medium-severity browser,
body-admission, carrier-filesystem, and legacy-archive-filesystem failures.
The analysis groups them by control ownership rather than severity.

Limitations: TAC connector availability could not be verified. No latency,
memory, or device-filesystem benchmark was performed for the architectural
options. Linux is the validated descriptor-anchored `os.Root` platform;
documented weaker Plan 9 and js semantics are outside the supported v0.7
runtime set.
