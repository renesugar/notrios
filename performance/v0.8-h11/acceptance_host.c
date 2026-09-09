/*
** Android acceptance host for the version-1 Notrios ABI (v0.8 H11).
**
** This is the same kind of program as cmd/notrioslib/hosttest/host_test.c and a
** different claim. That one proves the boundary holds when a C caller links the
** library on the machine that built it. This one runs on an Android emulator,
** against a library cross-compiled with the NDK, over a database file written
** by the desktop build — so what it can fail on is everything the desktop
** cannot: a different libc, a different filesystem, an SELinux policy, a
** process the system may kill, and a device that reboots.
**
** It is not an Android product and does not become one by passing. There is no
** APK, no UI, no Play Services, no background work and no secure store: the
** credential provider is supplied by the host that pushes the files here, which
** H11 records as a documented gap rather than a capability.
**
** Phases are selected by argv so the runner can stop the process between them
** and prove the library survives being killed rather than closed.
*/

#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "notrios_abi.h"

enum {
    OK = 0,
    INVALID_ARGUMENT = 1,
    INVALID_HANDLE = 3,
    STALE_HANDLE = 4,
    NOT_FOUND = 5,
    CONFLICT = 6,
    /* Copied from internal/abi/status.go rather than counted by eye. The first
    ** draft of this file put CANCELLED at 9, which is `unavailable`: a wrong
    ** constant in an acceptance host does not fail, it passes for the wrong
    ** reason, and the check that would have caught it happened to succeed. */
    PRECONDITION_REQUIRED = 7,
    CANCELLED = 11,
    END_OF_STREAM = 12,
    WOULD_BLOCK = 13
};

static int failures = 0;
static int checks = 0;

static void check(int condition, const char *what) {
    checks++;
    if (condition) {
        printf("  ok    %s\n", what);
    } else {
        printf("  FAIL  %s\n", what);
        failures++;
    }
}

/* A failure that reports only "FAIL" cannot be acted on from a log pulled off a
** device, so a status check prints the code it actually got. */
static void check_status(int32_t got, int32_t want, const char *what) {
    checks++;
    if (got == want) {
        printf("  ok    %s\n", what);
    } else {
        printf("  FAIL  %s (status %d, wanted %d)\n", what, (int)got, (int)want);
        failures++;
    }
}

static int32_t run_call(uint64_t instance, const char *request,
                        unsigned char **out, size_t *out_len) {
    uint64_t call = 0;
    int32_t rc = notrios_call_start(instance, (const unsigned char *)request,
                                    strlen(request), &call);
    if (rc != OK) {
        return rc;
    }
    for (;;) {
        rc = notrios_call_poll(instance, call, out, out_len);
        if (rc != WOULD_BLOCK) {
            return rc;
        }
    }
}

/* Extracts one JSON string field, which is all this host needs to read. It is
** deliberately not a JSON parser: a test that needs one is a test that has
** started asserting on the shape of the response rather than on the boundary. */
static int field(const char *body, const char *key, char *out, size_t cap) {
    char needle[64];
    snprintf(needle, sizeof needle, "\"%s\":\"", key);
    const char *start = strstr(body, needle);
    if (!start) return 0;
    start += strlen(needle);
    const char *end = strchr(start, '"');
    if (!end || (size_t)(end - start) >= cap) return 0;
    memcpy(out, start, (size_t)(end - start));
    out[end - start] = '\0';
    return 1;
}

/* ---- phase: open, identity, and the one-owner rule ---- */
static void phase_open(const char *profile) {
    check(notrios_abi_version() == 1, "abi version is 1 on Android");
    check(notrios_capabilities() == 0x1f,
          "capabilities negotiate to the frozen 0x1f: polling, streams, events, cancellation, generation handles");

    uint64_t instance = 0;
    check(notrios_instance_open(profile, strlen(profile), &instance) == OK && instance != 0,
          "instance opened on a database the desktop wrote");

    uint64_t second = 0;
    check(notrios_instance_open(profile, strlen(profile), &second) == CONFLICT,
          "a second owner of the same file is refused inside one process");

    unsigned char *data = NULL;
    size_t length = 0;
    check(run_call(instance, "{\"op\":\"abi.info\"}", &data, &length) == OK,
          "abi.info answers");
    if (data) {
        check(strstr((const char *)data, "\"abi_major\":1") != NULL, "abi.info reports major 1");
        notrios_buffer_release(instance, data, length);
    }
    check(notrios_instance_close(instance) == OK, "instance closed");
    check(notrios_instance_close(instance) == STALE_HANDLE, "the closed handle is stale, not merely invalid");
}

/* ---- phase: what the desktop left, and what Android adds ---- */
static void phase_work(const char *profile, const char *seeded_title) {
    uint64_t instance = 0;
    if (notrios_instance_open(profile, strlen(profile), &instance) != OK) {
        check(0, "instance opened for the work phase");
        return;
    }
    unsigned char *data = NULL;
    size_t length = 0;
    int32_t rc = OK;

    /* FTS5 over rows the desktop build wrote. A search that finds nothing here
    ** would mean the index did not survive the file crossing machines. */
    char request[512];
    snprintf(request, sizeof request,
             "{\"op\":\"search\",\"payload\":{\"collection_id\":\"default\",\"query\":\"%s\",\"limit\":10}}",
             seeded_title);
    check(run_call(instance, request, &data, &length) == OK, "FTS5 search runs");
    check(data && strstr((const char *)data, seeded_title) != NULL,
          "FTS5 finds the note the desktop seeded");
    if (data) { notrios_buffer_release(instance, data, length); data = NULL; }

    /* Create, read back, update, and read the revision the update made. */
    check(run_call(instance,
                   "{\"op\":\"note.create\",\"payload\":{\"collection_id\":\"default\","
                   "\"title\":\"written on Android\",\"body\":\"emulator body\","
                   "\"mime_type\":\"text/markdown\"}}", &data, &length) == OK,
          "note created on the emulator");
    char created[128] = {0};
    char revision[128] = {0};
    if (data) {
        check(field((const char *)data, "id", created, sizeof created), "the created note has an id");
        check(field((const char *)data, "revision_id", revision, sizeof revision),
              "the created note names the revision it is at");
        notrios_buffer_release(instance, data, length); data = NULL;
    }

    snprintf(request, sizeof request, "{\"op\":\"note.get\",\"payload\":{\"id\":\"%s\"}}", created);
    check(run_call(instance, request, &data, &length) == OK, "note read back");
    if (data) { notrios_buffer_release(instance, data, length); data = NULL; }

    /* An update names the revision it is replacing. Sending none is a real
    ** thing a host will do by mistake, so what comes back is worth recording
    ** rather than avoiding. */
    snprintf(request, sizeof request,
             "{\"op\":\"note.update\",\"payload\":{\"id\":\"%s\",\"title\":\"written on Android\","
             "\"body\":\"no revision named\",\"mime_type\":\"text/markdown\"}}", created);
    check_status(run_call(instance, request, &data, &length), PRECONDITION_REQUIRED,
                 "an update naming no base revision is refused as precondition_required, "
                 "not as an internal error");
    if (data) { notrios_buffer_release(instance, data, length); data = NULL; }

    snprintf(request, sizeof request,
             "{\"op\":\"note.update\",\"payload\":{\"id\":\"%s\",\"revision_id\":\"%s\","
             "\"title\":\"written on Android\",\"body\":\"emulator body, revised\","
             "\"mime_type\":\"text/markdown\"}}", created, revision);
    check_status(run_call(instance, request, &data, &length), OK, "note updated against its base revision");
    if (data) { notrios_buffer_release(instance, data, length); data = NULL; }

    snprintf(request, sizeof request, "{\"op\":\"note.revisions\",\"payload\":{\"id\":\"%s\"}}", created);
    check(run_call(instance, request, &data, &length) == OK, "revisions listed");
    if (data) {
        check(strstr((const char *)data, "rev_") != NULL, "the update wrote a revision");
        notrios_buffer_release(instance, data, length); data = NULL;
    }

    /* Cancellation: a finished call cannot be cancelled, and an unknown one is
    ** refused rather than silently accepted. Both are the host contract. */
    uint64_t call = 0;
    const char *info = "{\"op\":\"abi.info\"}";
    check(notrios_call_start(instance, (const unsigned char *)info, strlen(info), &call) == OK,
          "call started for the cancellation probe");
    check_status(notrios_call_cancel(instance, call + 4096), INVALID_HANDLE,
                 "cancelling a call that does not exist is refused");
    notrios_call_cancel(instance, call);
    /* Polled to completion rather than once: a cancelled call is still a call,
    ** and would_block means the answer has not arrived yet rather than that
    ** there will not be one. Polling once and reading would_block as a verdict
    ** is the mistake this loop exists to not make. */
    for (;;) {
        rc = notrios_call_poll(instance, call, &data, &length);
        if (rc != WOULD_BLOCK) break;
    }
    check(rc == OK || rc == CANCELLED,
          "a cancelled call answers with its result or with cancelled, never with nothing");
    if (rc == OK && data) { notrios_buffer_release(instance, data, length); data = NULL; }

    check(notrios_event_poll(instance, &data, &length) == WOULD_BLOCK,
          "an empty event queue reports would_block rather than blocking the thread");

    notrios_instance_close(instance);
}

/* ---- phase: a bounded stream over bytes the desktop put in the library ---- */
static void phase_stream(const char *profile, const char *resource_id, size_t expected_bytes) {
    uint64_t instance = 0;
    if (notrios_instance_open(profile, strlen(profile), &instance) != OK) {
        check(0, "instance opened for the stream phase");
        return;
    }
    unsigned char *data = NULL;
    size_t length = 0;
    char request[256];

    snprintf(request, sizeof request, "{\"op\":\"resource.get\",\"payload\":{\"id\":\"%s\"}}", resource_id);
    check(run_call(instance, request, &data, &length) == OK, "resource metadata read");
    if (data) { notrios_buffer_release(instance, data, length); data = NULL; }

    /* `max_bytes` at open is the budget for the whole stream, and the size
    ** passed to each read is the size of that read. Two different bounds, and
    ** the first draft of this host conflated them -- it opened with a 64-byte
    ** budget, read 64 bytes of a 149-byte resource, and reported that the
    ** library had lost 85 bytes. Both are asserted here so the distinction
    ** cannot quietly go back to being one thing. */
    uint64_t stream = 0;
    snprintf(request, sizeof request, "{\"resource_id\":\"%s\",\"max_bytes\":%zu}",
             resource_id, expected_bytes);
    check(notrios_stream_open(instance, (const unsigned char *)request, strlen(request), &stream) == OK
          && stream != 0, "stream opened over the seeded resource");

    size_t total = 0;
    int reads = 0;
    for (;;) {
        int32_t rc = notrios_stream_read(instance, stream, 64, &data, &length);
        if (rc == END_OF_STREAM) break;
        if (rc != OK) { check_status(rc, OK, "stream read returned a usable status"); break; }
        check(length <= 64, "a bounded read never returns more than it was asked for");
        total += length;
        reads++;
        notrios_buffer_release(instance, data, length);
        data = NULL;
        if (reads > 10000) { check(0, "stream terminated"); break; }
    }
    check(total == expected_bytes, "the stream delivered exactly the bytes the desktop stored");
    check(reads > 1, "the bytes arrived in several bounded reads rather than one buffer");
    check(notrios_stream_close(instance, stream) == OK, "stream closed");
    check_status(notrios_stream_close(instance, stream), STALE_HANDLE, "the closed stream handle is stale");

    /* The budget is the other bound, and it is the one that matters on a phone:
    ** a host that asks for 32 bytes of a large resource is answered with 32 and
    ** the stream ends, rather than the device reading the whole file. */
    uint64_t bounded = 0;
    snprintf(request, sizeof request, "{\"resource_id\":\"%s\",\"max_bytes\":32}", resource_id);
    check(notrios_stream_open(instance, (const unsigned char *)request, strlen(request), &bounded) == OK,
          "stream opened with a budget smaller than the resource");
    size_t budgeted = 0;
    for (;;) {
        int32_t rc = notrios_stream_read(instance, bounded, 64, &data, &length);
        if (rc != OK) break;
        budgeted += length;
        notrios_buffer_release(instance, data, length);
        data = NULL;
    }
    check(budgeted == 32, "a stream budget stops the read at what the host asked for");
    notrios_stream_close(instance, bounded);

    notrios_instance_close(instance);
}

/* ---- phase: write and then die, so the runner can prove recovery ---- */
static void phase_abandon(const char *profile) {
    uint64_t instance = 0;
    if (notrios_instance_open(profile, strlen(profile), &instance) != OK) {
        check(0, "instance opened for the abandon phase");
        return;
    }
    unsigned char *data = NULL;
    size_t length = 0;
    if (run_call(instance,
                 "{\"op\":\"note.create\",\"payload\":{\"collection_id\":\"default\","
                 "\"title\":\"committed before the kill\",\"body\":\"this note was committed\","
                 "\"mime_type\":\"text/markdown\"}}", &data, &length) == OK && data) {
        notrios_buffer_release(instance, data, length);
    }
    printf("  ok    a note was committed and the process is about to be killed\n");
    fflush(stdout);
    /* No close, no unlink, no checkpoint: exactly what a process killed by the
    ** system leaves behind. The runner sends SIGKILL after seeing this line. */
    for (;;) sleep(1);
}

/* ---- phase: what is still there afterwards ---- */
static void phase_verify(const char *profile, const char *expect_title, int expect_present) {
    uint64_t instance = 0;
    if (notrios_instance_open(profile, strlen(profile), &instance) != OK) {
        check(0, "instance opened for the verify phase");
        return;
    }
    unsigned char *data = NULL;
    size_t length = 0;
    char request[512];
    snprintf(request, sizeof request,
             "{\"op\":\"search\",\"payload\":{\"collection_id\":\"default\",\"query\":\"%s\",\"limit\":10}}",
             expect_title);
    check(run_call(instance, request, &data, &length) == OK, "search runs against the reopened database");
    if (data) {
        int found = strstr((const char *)data, expect_title) != NULL;
        check(found == expect_present, expect_present
              ? "work committed before the process died is present after reopening"
              : "nothing uncommitted appeared after reopening");
        notrios_buffer_release(instance, data, length);
    }
    notrios_instance_close(instance);
}

int main(int argc, char **argv) {
    if (argc < 3) {
        fprintf(stderr, "usage: %s <phase> <profile-path> [args...]\n", argv[0]);
        return 2;
    }
    const char *phase = argv[1];
    const char *profile = argv[2];

    if (strcmp(phase, "open") == 0) {
        phase_open(profile);
    } else if (strcmp(phase, "work") == 0 && argc >= 4) {
        phase_work(profile, argv[3]);
    } else if (strcmp(phase, "stream") == 0 && argc >= 5) {
        phase_stream(profile, argv[3], (size_t)strtoul(argv[4], NULL, 10));
    } else if (strcmp(phase, "abandon") == 0) {
        phase_abandon(profile);
    } else if (strcmp(phase, "verify") == 0 && argc >= 5) {
        phase_verify(profile, argv[3], atoi(argv[4]));
    } else {
        fprintf(stderr, "unknown or incomplete phase: %s\n", phase);
        return 2;
    }

    printf("%s: %d checks, %d failures\n", phase, checks, failures);
    return failures == 0 ? 0 : 1;
}
