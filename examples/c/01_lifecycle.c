/*
** Lifecycle: open a library, ask it something, close it.
**
** The first thing any host does, and the first thing that can go wrong. Note
** the order: check the versions agree before opening anything, because a
** mismatched header and library disagree about every call after this one.
*/
#include <stdio.h>
#include <string.h>

#include "notrios_abi.h"

enum { OK = 0, WOULD_BLOCK = 13 };

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <library-path>\n", argv[0]);
        return 2;
    }

    if (notrios_abi_version() != NOTRIOS_ABI_VERSION) {
        fprintf(stderr, "header says ABI %u, library says %u; refusing to continue\n",
                NOTRIOS_ABI_VERSION, notrios_abi_version());
        return 1;
    }

    uint64_t instance = 0;
    int32_t rc = notrios_instance_open(argv[1], strlen(argv[1]), &instance);
    if (rc != OK) {
        fprintf(stderr, "open failed: %d\n", rc);
        return 1;
    }

    /* A call is started and then polled. Poll returns would-block rather than
       blocking, so a single-threaded host stays responsive. */
    uint64_t call = 0;
    const char *request = "{\"op\":\"abi.info\"}";
    rc = notrios_call_start(instance, (const unsigned char *)request, strlen(request), &call);
    if (rc != OK) {
        fprintf(stderr, "call_start failed: %d\n", rc);
        notrios_instance_close(instance);
        return 1;
    }

    unsigned char *body = NULL;
    size_t length = 0;
    do {
        rc = notrios_call_poll(instance, call, &body, &length);
    } while (rc == WOULD_BLOCK);
    if (rc != OK) {
        fprintf(stderr, "call_poll failed: %d\n", rc);
        notrios_instance_close(instance);
        return 1;
    }

    printf("abi.info: %.*s\n", (int)length, (const char *)body);
    notrios_buffer_release(instance, body, length);

    if (notrios_instance_close(instance) != OK) {
        fprintf(stderr, "close failed\n");
        return 1;
    }
    printf("lifecycle: ok\n");
    return 0;
}
