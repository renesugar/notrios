/*
** Streams: reading bytes that do not fit in one call.
**
** A stream is opened, read in bounded chunks until end_of_stream, and closed.
** Each chunk is a library-owned buffer, released like any other. This example
** first creates a note through the ordinary call path, then streams a resource
** that does not exist, because the refusal is the part a host must handle:
** a missing resource produces no handle at all.
*/
#include <stdio.h>
#include <string.h>

#include "notrios_abi.h"

enum { OK = 0, NOT_FOUND = 5, END_OF_STREAM = 12, WOULD_BLOCK = 13 };

static int failures = 0;

static void expect(int condition, const char *what) {
    printf("  %-4s %s\n", condition ? "ok" : "FAIL", what);
    if (!condition) {
        failures++;
    }
}

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <library-path>\n", argv[0]);
        return 2;
    }
    uint64_t instance = 0;
    if (notrios_instance_open(argv[1], strlen(argv[1]), &instance) != OK) {
        fprintf(stderr, "open failed\n");
        return 1;
    }

    /* A stream over something that is not there refuses, and leaves no handle
       for a host to close or leak. */
    uint64_t stream = 0;
    const char *missing = "{\"resource_id\":\"res_missing\",\"max_bytes\":64}";
    int32_t rc = notrios_stream_open(instance, (const unsigned char *)missing, strlen(missing), &stream);
    expect(rc == NOT_FOUND && stream == 0, "a stream over a missing resource is refused with no handle");

    /* An empty event queue reports would_block rather than blocking, which is
       what lets a host poll it from its own loop. */
    unsigned char *body = NULL;
    size_t length = 0;
    expect(notrios_event_poll(instance, &body, &length) == WOULD_BLOCK,
           "an empty event queue reports would_block");

    /* Reading a stream: the shape a host writes, with the chunk released each
       time round the loop. Shown against the refusal above so the example
       needs no attachment in the library it is run against. */
    if (stream != 0) {
        for (;;) {
            unsigned char *chunk = NULL;
            size_t chunk_length = 0;
            rc = notrios_stream_read(instance, stream, 4096, &chunk, &chunk_length);
            if (rc == END_OF_STREAM) {
                break;
            }
            if (rc != OK) {
                failures++;
                break;
            }
            notrios_buffer_release(instance, chunk, chunk_length);
        }
        notrios_stream_close(instance, stream);
    }

    notrios_instance_close(instance);
    printf("streams: %s\n", failures == 0 ? "ok" : "FAILED");
    return failures == 0 ? 0 : 1;
}
