/*
** Errors: every failure is a code you can act on, not a crash or a guess.
**
** The codes below are the ones a host meets in practice. A request that is not
** understood fails at runtime with not_found; it does not fail to link, and it
** does not take the instance down with it.
*/
#include <stdio.h>
#include <string.h>

#include "notrios_abi.h"

enum {
    OK = 0,
    INVALID_ARGUMENT = 1,
    INVALID_HANDLE = 3,
    STALE_HANDLE = 4,
    NOT_FOUND = 5,
    CONFLICT = 6,
    WOULD_BLOCK = 13
};

static int failures = 0;

static void expect(int32_t got, int32_t want, const char *what) {
    printf("  %-4s %s (rc=%d)\n", got == want ? "ok" : "FAIL", what, got);
    if (got != want) {
        failures++;
    }
}

static int32_t call(uint64_t instance, const char *request, unsigned char **body, size_t *length) {
    uint64_t handle = 0;
    int32_t rc = notrios_call_start(instance, (const unsigned char *)request, strlen(request), &handle);
    if (rc != OK) {
        return rc;
    }
    do {
        rc = notrios_call_poll(instance, handle, body, length);
    } while (rc == WOULD_BLOCK);
    return rc;
}

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <library-path>\n", argv[0]);
        return 2;
    }
    uint64_t instance = 0;
    expect(notrios_instance_open(argv[1], strlen(argv[1]), &instance), OK, "open");

    unsigned char *body = NULL;
    size_t length = 0;

    expect(call(instance, "{\"op\":\"nope\"}", &body, &length), NOT_FOUND,
           "an unknown operation is a runtime not_found");
    if (body) {
        notrios_buffer_release(instance, body, length);
        body = NULL;
    }

    expect(call(instance, "not json at all", &body, &length), INVALID_ARGUMENT,
           "a malformed request is invalid_argument");
    if (body) {
        notrios_buffer_release(instance, body, length);
        body = NULL;
    }

    uint64_t second = 0;
    expect(notrios_instance_open(argv[1], strlen(argv[1]), &second), CONFLICT,
           "a second owner of the same library is refused");

    expect(notrios_call_poll(instance, 0, &body, &length), INVALID_HANDLE,
           "polling a zero call handle is invalid_handle");

    notrios_instance_close(instance);
    expect(call(instance, "{\"op\":\"abi.info\"}", &body, &length), STALE_HANDLE,
           "a call on a closed instance is stale_handle");

    printf("errors: %s\n", failures == 0 ? "ok" : "FAILED");
    return failures == 0 ? 0 : 1;
}
