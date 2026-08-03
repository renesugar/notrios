# v0.4 Q1 Boolean-search profile

This directory records aggregate-only evidence from the generated 10,000-note
H7 corpus after Q1 added bounded boolean search expressions and the
`category:` notebook alias.

The executable harness is unchanged:

```bash
bash scripts/run_large_library_profile.sh 10000 /tmp/notrios-v0.4-q1-10k.json
```

In addition to the established keyset-page checks, the profile asserts exact
result sets for `OR` precedence, grouped field negation, and equivalence of
`category:` and recursive `notebook:` matching. It measures representative
text-OR, category-alias, and deliberately nonselective negated-field queries.
The latter is reported honestly at 288.542 ms p95 on this reference machine;
it is not an ordinary All-notes page and is not claimed to meet the 100 ms
ordinary-page target. All ordinary first/next/deep pages remained below that
target.
