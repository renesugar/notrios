# G1a provenance and license record

The implementation was written spec-first in Go for this investigation. The
listed projects served as specifications, behavioral references, or external
test oracles; none is linked, vendored, or added to `go.mod`.

| Reference | Exact revision / document | License | G1a use |
|---|---|---|---|
| RFC 3284, *The VCDIFF Generic Differencing and Compression Data Format* | RFC 3284 | IETF RFC text | File/window grammar, base-128 integer encoding, default-code-table semantics, source/target address space, and overlap behavior. |
| Apache Subversion `subversion/libsvn_delta/xdelta.c` | `3a113b9b5e8050df2dbb32283b21c00934976711` | Apache-2.0 | Behavioral comparison for 64-byte source blocks, rolling pseudo-Adler matching, byte confirmation, and match extension. Not copied or translated line by line. |
| Apache Subversion `subversion/libsvn_delta/svndiff.c` | `3a113b9b5e8050df2dbb32283b21c00934976711` | Apache-2.0 | Confirmed that svndiff is a distinct Subversion serialization. G1a does not emit or claim byte compatibility with it. |
| google/open-vcdiff | `868f459a8d815125c2457f8c74b12493853100f9` | Apache-2.0 | C++ external interoperability oracle, including its pinned gflags submodule. Test-only `/tmp` build. |
| jmacd/xdelta (xdelta3) | `9822b17313263d458b80511b08124971fc0e04fa` | Apache-2.0 | C external interoperability oracle. Test-only `/tmp` build with LZMA and armor disabled. |
| antmicro/go-xdelta | `9180b718329b756354916b18c6ca834bbc21722c` | Apache-2.0 | Confirmed that the available Go interface wraps C/cgo and therefore does not satisfy the pure-Go runtime requirement. No dependency or source reuse. |
| epiclabs-io/diff3 | `3b1669897fb1aa7c1fb2699a3c6a45bbb46e9ec1` | MIT | G1 text three-way-merge probe only. It is not a binary delta codec and is not a G1a runtime candidate. |

No GPL source was consulted or used to derive code. The historical GPL xdelta
implementations named in background research were not implementation
references. The external xdelta3 oracle above is the current
Apache-2.0 repository revision recorded in the table. G1a contains no copied C
or C++ and requires no NOTICE addition because it does not distribute or
translate those implementations. Behavioral attribution remains here and in
the package documentation.

The resulting prototype itself is repository code under Notrios's Apache-2.0
license. Any later promotion must repeat the provenance review and decide
whether the production implementation is an independent rewrite or a reviewed
promotion of this prototype.
