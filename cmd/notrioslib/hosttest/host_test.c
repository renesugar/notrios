/*
** C host acceptance test for the version-1 Notrios ABI.
**
** This is the only place the ABI is exercised the way a real host does: as C,
** through the generated header, against a built shared library. The Go tests
** in internal/abi prove the logic; this proves the boundary — that the symbols
** link, the memory rules hold, and a caller using nothing but notrios_abi.h can
** open a library, run a call, read a stream, and shut down.
**
** Built and run by run_host_test.sh.
*/

#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "notrios_abi.h"

enum {
    OK = 0,
    INVALID_ARGUMENT = 1,
    INVALID_HANDLE = 3,
    STALE_HANDLE = 4,
    NOT_FOUND = 5,
    CONFLICT = 6,
    END_OF_STREAM = 12,
    WOULD_BLOCK = 13
};

static int failures = 0;

static void check(int condition, const char *what) {
    if (condition) {
        printf("  ok    %s\n", what);
    } else {
        printf("  FAIL  %s\n", what);
        failures++;
    }
}

/* Run one request to completion, polling the way a host must. */
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

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <profile-path>\n", argv[0]);
        return 2;
    }
    const char *profile = argv[1];

    check(notrios_abi_version() == 1, "abi version is 1");
    check(notrios_capabilities() == 0x1f, "capabilities are the frozen 0x1f");

    /* A zero handle must never resolve. */
    check(notrios_instance_close(0) == INVALID_HANDLE, "zero instance handle refused");

    uint64_t instance = 0;
    int32_t rc = notrios_instance_open(profile, strlen(profile), &instance);
    check(rc == OK && instance != 0, "instance opened");
    if (rc != OK) {
        return 1;
    }

    /* One owner: a second open of the same path is refused. */
    uint64_t second = 0;
    check(notrios_instance_open(profile, strlen(profile), &second) == CONFLICT,
          "second owner of the same database refused");

    unsigned char *data = NULL;
    size_t length = 0;
    rc = run_call(instance, "{\"op\":\"abi.info\"}", &data, &length);
    check(rc == OK && data != NULL && length > 0, "abi.info returned a body");
    check(strstr((const char *)data, "\"abi_major\":1") != NULL,
          "abi.info reports major 1");
    check(notrios_buffer_release(instance, data, length) == OK, "buffer released");
    /* Releasing the same pointer twice must be refused, not acted on. */
    check(notrios_buffer_release(instance, data, length) == INVALID_ARGUMENT,
          "double release refused");
    data = NULL;
    length = 0;

    /* A foreign pointer must never be freed by this library. */
    unsigned char *foreign = (unsigned char *)malloc(16);
    check(notrios_buffer_release(instance, foreign, 16) == INVALID_ARGUMENT,
          "foreign pointer release refused");
    free(foreign);

    rc = run_call(instance,
                  "{\"op\":\"note.create\",\"payload\":{\"collection_id\":\"default\","
                  "\"title\":\"host note\",\"body\":\"written from C\","
                  "\"mime_type\":\"text/markdown\"}}",
                  &data, &length);
    check(rc == OK, "note created through the ABI");
    check(data != NULL && strstr((const char *)data, "written from C") != NULL,
          "created note echoed its body");
    if (data) {
        notrios_buffer_release(instance, data, length);
        data = NULL;
    }

    /* An unknown operation is a runtime not_found, not a link failure. */
    rc = run_call(instance, "{\"op\":\"nope\"}", &data, &length);
    check(rc == NOT_FOUND, "unknown operation reported as not_found");
    if (data) {
        notrios_buffer_release(instance, data, length);
        data = NULL;
    }

    /* Streams: a missing resource refuses without producing a handle. */
    uint64_t stream = 0;
    const char *stream_request = "{\"resource_id\":\"res_missing\",\"max_bytes\":16}";
    rc = notrios_stream_open(instance, (const unsigned char *)stream_request,
                             strlen(stream_request), &stream);
    check(rc == NOT_FOUND && stream == 0, "stream over a missing resource refused");

    /* Events: an empty queue reports would_block rather than blocking. */
    rc = notrios_event_poll(instance, &data, &length);
    check(rc == WOULD_BLOCK, "empty event queue reports would_block");

    check(notrios_instance_close(instance) == OK, "instance closed");
    /* After close the handle is detectably stale, not merely invalid. */
    check(notrios_instance_close(instance) == STALE_HANDLE,
          "closed instance handle is stale");
    rc = run_call(instance, "{\"op\":\"abi.info\"}", &data, &length);
    check(rc == STALE_HANDLE, "call on a closed instance is stale");

    /* Ownership was released, so the database can be opened again. */
    uint64_t reopened = 0;
    check(notrios_instance_open(profile, strlen(profile), &reopened) == OK,
          "database reopened after close released ownership");
    notrios_instance_close(reopened);

    printf("%s\n", failures == 0 ? "host test: PASS" : "host test: FAIL");
    return failures == 0 ? 0 : 1;
}
