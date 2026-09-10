# v0.9 I5 — what signing means, per platform

Nothing signs a release today. `build_deb.sh` and `package_release.sh` produce
unsigned artifacts, which is exactly what v0.8 allowed for internal evidence.
This item decides what signing *will* mean, so that I6 can implement it under
constraints that already exist rather than inventing them at the point of use.

```sh
python3 performance/v0.9-i5/validate_evidence.py     # gates the record
python3 performance/v0.9-i5/check_secret_exposure.py # gates the workflows
```

## No key was created, deliberately

Creating a production signing key before deciding what it is for, whose it is,
where it lives and how it rotates would invert the work. The policy decides
those; the key is a consequence, and I6 is where one first appears.

## The one thing that is enforced rather than promised

`check_secret_exposure.py` refuses a workflow that could hand signing material
to a pull request. This matters concretely here: `ci.yml` runs on both `push`
and `pull_request`, and **a same-repository pull request receives repository
secrets** — so a repository-level signing key would be readable from any branch
anyone pushes. Hence: environment secrets with required reviewers, never
repository secrets.

Three rules, each tried against a deliberately bad workflow: a secret in a
workflow with a `pull_request` trigger, `pull_request_target` at all, and a
secret used with no `environment` to gate it. `GITHUB_TOKEN` is exempt — it is
issued per run and scoped by `permissions:`.

It reads workflow files as text, so `make validate` gains no YAML dependency.
That makes it per-file rather than per-job, which over-approximates on purpose:
a gate that is too coarse produces an argument, one that is too clever produces
a leak.

## The release key is not the evidence key

The reserve's key exists so the archive's integrity is independent of
everything else. A release key is used by automation, on shipping's schedule,
exposed to a build pipeline. Sharing one would make the archive only as
trustworthy as the release pipeline's worst day.

## Where each platform stands

| Platform | Status | Blocked on |
| --- | --- | --- |
| Ubuntu 24.04 amd64 | available | — (installed and run in I3) |
| Ubuntu 24.04 arm64 | blocked | no arm64 machine; still level 2 |
| Windows amd64 | blocked | an OV/EV certificate, and a native runner |
| macOS | blocked | an Apple Developer account, and native hardware |

Signing an artifact nobody has run would attest its origin and say nothing
about whether it works, which is why the arm64 entry is blocked on a machine
rather than on a key.

## Derived, not restated

The RFC 3161 policy OID comes from `evidence/verify_evidence.py` — the code that
will actually reject a mismatched timestamp — and the Ubuntu amd64 claim level
comes from I3's report, which is what installed and ran the package. Both are
checked, so neither can drift into a second version of the truth.
