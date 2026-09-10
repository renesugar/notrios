/* I8 — the C ABI at its edges, under a memory checker.
**
** H11 proved a C host can drive the library. This one tries to break it:
** handles that are wrong, stale or never issued; a buffer released twice; an
** instance closed while a call is in flight; a stream read past its budget.
** The question is not whether these succeed -- they must not -- but whether
** they fail *cleanly*, with a status code, rather than by corrupting memory.
**
** It is built to run under valgrind, because the failure this is really about
** does not announce itself. A double free that happens to land on a free list
** returns a status code and passes; the same call under memcheck says exactly
** what happened. A test for ownership bugs that does not run under a memory
** checker is a test that agrees with the bug.
**
** The error constants are copied from internal/abi/status.go rather than
** counted by eye -- H11's first draft put CANCELLED at 9, which is
** `unavailable`, and it passed. */
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "notrios_abi.h"

enum {
    OK = 0,
    INVALID_ARGUMENT = 1,
    INVALID_HANDLE = 3,
    STALE_HANDLE = 4,
    PRECONDITION_REQUIRED = 7,
    CANCELLED = 11,
    END_OF_STREAM = 12,
    WOULD_BLOCK = 13
};

static int failures = 0;
static int checks = 0;

static void check(int condition, const char *what) {
    checks++;
    printf(condition ? "  ok    %s\n" : "  FAIL  %s\n", what);
    if (!condition) failures++;
}

/* Any refusal will do, as long as it is a refusal. Pinning one code here would
** freeze an implementation detail the ABI does not promise; what the ABI
** promises is that a bad handle is rejected rather than followed. */
static void check_refused(int32_t got, const char *what) {
    checks++;
    if (got != OK) {
        printf("  ok    %s (refused with %d)\n", what, (int)got);
    } else {
        printf("  FAIL  %s (accepted)\n", what);
        failures++;
    }
}

static int32_t run_call(uint64_t instance, const char *request,
                        unsigned char **out, size_t *out_len) {
    uint64_t call = 0;
    int32_t rc = notrios_call_start(instance, (const unsigned char *)request,
                                    strlen(request), &call);
    if (rc != OK) return rc;
    for (;;) {
        rc = notrios_call_poll(instance, call, out, out_len);
        if (rc != WOULD_BLOCK) return rc;
    }
}

struct racer {
    uint64_t instance;
    int32_t last;
};

static void *hammer(void *argument) {
    struct racer *state = (struct racer *)argument;
    unsigned char *data = NULL;
    size_t length = 0;
    for (int i = 0; i < 40; i++) {
        state->last = run_call(state->instance, "{\"op\":\"abi.info\"}", &data, &length);
        if (state->last == OK && data != NULL) {
            notrios_buffer_release(state->instance, data, length);
            data = NULL;
        }
    }
    return NULL;
}

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <profile-directory>\n", argv[0]);
        return 2;
    }
    const char *profile = argv[1];
    uint64_t instance = 0;
    unsigned char *data = NULL;
    size_t length = 0;

    printf("abi version %u\n", (unsigned)notrios_abi_version());

    if (notrios_instance_open(profile, strlen(profile), &instance) != OK) {
        fprintf(stderr, "could not open the instance\n");
        return 1;
    }

    /* 1. Handles that were never issued. */
    check_refused(notrios_call_poll(instance, 0xdeadbeefULL, &data, &length),
                  "poll a call handle that was never issued");
    check_refused(notrios_call_cancel(instance, 0xdeadbeefULL),
                  "cancel a call handle that was never issued");
    check_refused(notrios_stream_read(instance, 0xdeadbeefULL, 64, &data, &length),
                  "read a stream handle that was never issued");
    check_refused(notrios_stream_close(instance, 0xdeadbeefULL),
                  "close a stream handle that was never issued");
    check_refused(notrios_call_poll(0xbadf00dULL, 1, &data, &length),
                  "poll against an instance handle that was never issued");

    /* 2. A buffer released twice, and one that was never issued.
    **
    ** This is the check that most needs a memory checker: a second release of
    ** a pointer the library has already reclaimed is a double free, and a
    ** double free frequently *returns success*. */
    data = NULL; length = 0;
    if (run_call(instance, "{\"op\":\"abi.info\"}", &data, &length) == OK && data != NULL) {
        unsigned char *copy = data;
        size_t copy_len = length;
        check(notrios_buffer_release(instance, data, length) == OK, "a buffer is released once");
        check_refused(notrios_buffer_release(instance, copy, copy_len),
                      "the same buffer is refused a second release");
    } else {
        check(0, "could not obtain a buffer to release");
    }
    unsigned char stack_bytes[8] = {0};
    check_refused(notrios_buffer_release(instance, stack_bytes, sizeof stack_bytes),
                  "a pointer the library never issued is refused");

    /* 3. Cancellation, polled to a verdict.
    **
    ** WOULD_BLOCK is not an answer. H11 learned this: a cancelled call has to
    ** be polled until it says what happened, or the test records the absence
    ** of a result as a result. */
    uint64_t call = 0;
    if (notrios_call_start(instance, (const unsigned char *)"{\"op\":\"abi.info\"}",
                           strlen("{\"op\":\"abi.info\"}"), &call) == OK) {
        notrios_call_cancel(instance, call);
        /* A deadline that yields, not a spin that counts.
        **
        ** The first version polled ten thousand times in a tight loop and
        ** reported that a cancelled call never answered. It answers: the loop
        ** simply never let the runtime schedule the goroutine that would
        ** produce the verdict. Counting iterations measures this host's
        ** scheduling luck; a wall-clock deadline with a yield measures the
        ** library. H11's loop is unbounded for the same reason -- it just never
        ** had to say so. */
        int32_t verdict = WOULD_BLOCK;
        data = NULL; length = 0;
        struct timespec pause = {0, 200000}; /* 0.2 ms */
        for (int i = 0; i < 50000 && verdict == WOULD_BLOCK; i++) {
            verdict = notrios_call_poll(instance, call, &data, &length);
            if (verdict == WOULD_BLOCK) nanosleep(&pause, NULL);
        }
        check(verdict != WOULD_BLOCK, "a cancelled call reaches a verdict rather than blocking");
        printf("        (cancelled call answered with %d)\n", (int)verdict);
        if (verdict == OK && data != NULL) notrios_buffer_release(instance, data, length);
        check_refused(notrios_call_poll(instance, call, &data, &length),
                      "a finished call handle is not reusable");
    } else {
        check(0, "could not start a call to cancel");
    }

    /* 4. Concurrent callers on one instance. */
    struct racer a = {instance, OK}, b = {instance, OK};
    pthread_t ta, tb;
    pthread_create(&ta, NULL, hammer, &a);
    pthread_create(&tb, NULL, hammer, &b);
    pthread_join(ta, NULL);
    pthread_join(tb, NULL);
    check(a.last == OK && b.last == OK, "two threads drive one instance concurrently");

    /* 5. Closing while a call is in flight, then using what is left. */
    uint64_t inflight = 0;
    int32_t started = notrios_call_start(instance, (const unsigned char *)"{\"op\":\"abi.info\"}",
                                         strlen("{\"op\":\"abi.info\"}"), &inflight);
    check(notrios_instance_close(instance) == OK, "the instance closes with a call outstanding");
    if (started == OK) {
        data = NULL; length = 0;
        check_refused(notrios_call_poll(instance, inflight, &data, &length),
                      "an outstanding call is refused after its instance closed");
    }
    check_refused(notrios_instance_close(instance), "a closed instance is refused a second close");
    check_refused(notrios_call_start(instance, (const unsigned char *)"{\"op\":\"abi.info\"}",
                                     strlen("{\"op\":\"abi.info\"}"), &call),
                  "a closed instance refuses new calls");

    printf("\n%d checks, %d failures\n", checks, failures);
    return failures == 0 ? 0 : 1;
}
