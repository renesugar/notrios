/*
** Threading: the library is safe to call from several threads, and it never
** calls back into yours.
**
** Each thread here starts and polls its own call on one shared instance. The
** rule a host must keep is about handles, not locks: a call handle belongs to
** whoever started it, and polling someone else's is a programming error rather
** than a race the library will tidy up.
*/
#include <pthread.h>
#include <stdio.h>
#include <string.h>

#include "notrios_abi.h"

enum { OK = 0, WOULD_BLOCK = 13, THREADS = 4, CALLS_PER_THREAD = 8 };

struct worker {
    uint64_t instance;
    int failures;
};

static void *run(void *argument) {
    struct worker *worker = argument;
    const char *request = "{\"op\":\"abi.info\"}";
    for (int i = 0; i < CALLS_PER_THREAD; i++) {
        uint64_t call = 0;
        if (notrios_call_start(worker->instance, (const unsigned char *)request,
                               strlen(request), &call) != OK) {
            worker->failures++;
            continue;
        }
        unsigned char *body = NULL;
        size_t length = 0;
        int32_t rc = WOULD_BLOCK;
        while (rc == WOULD_BLOCK) {
            rc = notrios_call_poll(worker->instance, call, &body, &length);
        }
        if (rc != OK || body == NULL) {
            worker->failures++;
            continue;
        }
        /* Each thread releases what its own call produced. */
        if (notrios_buffer_release(worker->instance, body, length) != OK) {
            worker->failures++;
        }
    }
    return NULL;
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

    pthread_t threads[THREADS];
    struct worker workers[THREADS];
    for (int i = 0; i < THREADS; i++) {
        workers[i] = (struct worker){.instance = instance, .failures = 0};
        if (pthread_create(&threads[i], NULL, run, &workers[i]) != 0) {
            fprintf(stderr, "pthread_create failed\n");
            notrios_instance_close(instance);
            return 1;
        }
    }
    int failures = 0;
    for (int i = 0; i < THREADS; i++) {
        pthread_join(threads[i], NULL);
        failures += workers[i].failures;
    }

    notrios_instance_close(instance);
    printf("threading: %d threads x %d calls, %d failure(s)\n", THREADS, CALLS_PER_THREAD, failures);
    return failures == 0 ? 0 : 1;
}
