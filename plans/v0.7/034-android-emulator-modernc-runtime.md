# v0.7 G18 amendment — Android emulator modernc runtime

Date: 2026-08-26
Model: GPT-5 (exact serving variant unavailable)
Working state: complete runtime follow-up; no product dependency selected

## Goal and boundary

Determine whether the Android emulator tests missing from the modernc SQLite
evaluation can run after the Android SDK repair. Install the authorized system
images, configure an AVD, execute the exact disposable candidate, and preserve
machine-checkable evidence. This amendment does not add a product dependency,
change the canonical store, create a Flutter application, implement the planned
C ABI, claim upstream Android support, or authorize G18b.

## Environment and CLI findings

The PATH-selected Temurin OpenJDK 21 ran the new `android` CLI without setting
or referencing `JAVA_HOME`. The host has usable KVM acceleration. The system
now retains two AVDs: the modern CLI's API 36 Google Play `medium_phone` and an
API 35 Google APIs x86_64 Pixel 6 named `notrios_api35_x86_64`. The test AVD was
cold-booted headlessly and stopped cleanly after validation; no emulator is
left connected.

The installed new CLI uses slash-separated package identifiers such as
`system-images/android-35/google_apis/x86_64`; the legacy `avdmanager` still
uses semicolon-separated identifiers. `android sdk --licenses` is not a valid
command in this CLI version, and `android sdk install` exposes no
`--accept-licenses` option. Licenses were accepted by piping affirmative input
to the install command. `android emulator create medium_phone` is profile based
and selected its own API 36 image, so the explicit API 35 AVD used
`avdmanager`. The `android run` command deploys an existing APK; it does not
build an absent Flutter/Gradle project.

## Runtime result

The disposable probe pinned `modernc.org/sqlite` v1.57.0 and
`modernc.org/libc` v1.74.4. Android x86_64 requires Go external linking, so the
executable used NDK API 35 clang with `CGO_ENABLED=1`; dependency inspection
confirmed modernc selected generated Linux/amd64 sources and no modernc cgo
files. This is different from using the host SQLite headers or library.

On Android 15/API 35 x86_64, all measured checks passed:

- SQLite 3.53.3 reported `ENABLE_FTS5`, `THREADSAFE=1`, and
  `MUTEX_PTHREADS`;
- FTS5 insert/MATCH, JSON, WAL mode, and `integrity_check` passed;
- a close/reopen run retained the first row and added a second;
- the database SHA-256 remained unchanged across emulator reboot, after which
  a third row was added and verified;
- an NDK-built x86_64 `c-shared` library passed `dlopen` and `dlclose` through a
  small disposable C harness.

The executable SHA-256 is
`823c48cad0ad26430fffbd32ecdc362480977ba6536df1fbe53191ca66d265b8`;
the shared-library SHA-256 is
`ab0048f1d1290d6eaa846802c072081148fea6105b200cfdb7c6390cc5634f87`.
The database's pre-reboot SHA-256 is
`c5974438bd72e6017636e36a0d40347359b4d90c444a36da52ddc64450a38724`.
Disposable sources, APK-independent binaries, database, and emulator logs
remain outside the repository.

## Decision effect and remaining work

The exact modernc candidate is now runtime-feasible on the measured API 35
x86_64 emulator and may remain beside the pinned C amalgamation control in v0.8
H0. It is still not selected. H0 must exercise both candidates on the same
emulator with the real Notrios ABI, schema-v27 store, snapshots, synchronization,
failure recovery, single-owner lifecycle, and production-shaped workload. It
must measure latency, throughput, CPU, allocations, RSS, and package size.
Android arm64, physical devices, iOS, and the Flutter integration remain
unverified. Upstream still does not advertise Android as a supported modernc
target.

No additional user configuration is needed to reproduce the completed x86_64
emulator pre-gate. The two AVDs may be reused by H0.

## Evidence and validation

- Machine record: `performance/v0.7-g18/ANDROID_EMULATOR_FOLLOWUP.json`
- Updated matrix: `performance/v0.7-g18/ANDROID_FEASIBILITY.json`
- Updated candidate record:
  `performance/v0.7-g18/MODERNC_SQLITE_EVALUATION.json`
- G18 Python evidence suite: nine tests pass.
- G18 source/evidence validator passes with 109 API operations and 19 platform
  capabilities.
- Flutter Doctor reports no issues and both configured AVDs.

Primary command references:

- <https://developer.android.com/tools/agents/android-cli>
- <https://developer.android.com/studio/run/managing-avds>
- <https://developer.android.com/studio/command-line/avdmanager>
- <https://developer.android.com/studio/run/emulator-commandline>
