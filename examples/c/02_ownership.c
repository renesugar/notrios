/*
** Ownership: who frees what, and what happens when you get it wrong.
**
** Every buffer the library returns belongs to the library until you release it.
** The three mistakes below are refused rather than acted on, which is the
** difference between a bug you find now and a heap corruption you find later.
*/
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "notrios_abi.h"

enum { OK = 0, INVALID_ARGUMENT = 1, INVALID_HANDLE = 3, STALE_HANDLE = 4, WOULD_BLOCK = 13 };

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

    expect(notrios_instance_close(0) == INVALID_HANDLE, "a zero handle never resolves");

    uint64_t instance = 0;
    if (notrios_instance_open(argv[1], strlen(argv[1]), &instance) != OK) {
        fprintf(stderr, "open failed\n");
        return 1;
    }

    uint64_t call = 0;
    const char *request = "{\"op\":\"abi.info\"}";
    notrios_call_start(instance, (const unsigned char *)request, strlen(request), &call);
    unsigned char *body = NULL;
    size_t length = 0;
    int32_t rc = WOULD_BLOCK;
    while (rc == WOULD_BLOCK) {
        rc = notrios_call_poll(instance, call, &body, &length);
    }

    expect(notrios_buffer_release(instance, body, length) == OK, "the library's buffer is released once");
    expect(notrios_buffer_release(instance, body, length) == INVALID_ARGUMENT, "releasing it twice is refused");

    unsigned char *mine = malloc(16);
    expect(notrios_buffer_release(instance, mine, 16) == INVALID_ARGUMENT,
           "releasing a pointer the library did not produce is refused");
    free(mine); /* still yours: the library did not take it */

    notrios_instance_close(instance);
    expect(notrios_instance_close(instance) == STALE_HANDLE,
           "a closed handle is stale, not merely invalid");

    printf("ownership: %s\n", failures == 0 ? "ok" : "FAILED");
    return failures == 0 ? 0 : 1;
}
