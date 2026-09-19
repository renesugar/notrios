/*
** Compatibility: what a host does when the library is not the one it was
** built against.
**
** Two rules, and they are not symmetric. The ABI major must match exactly —
** a different major is a different interface, and the soname says so. The
** capability bits must not: a newer library may advertise capabilities this
** host has never heard of, and ignoring them is how old hosts keep working.
*/
#include <stdio.h>
#include <string.h>

#include "notrios_abi.h"

enum { OK = 0 };

/* The capabilities this host knows how to use. A real host lists its own. */
#define KNOWN_CAPABILITIES 0x1fu

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

    uint32_t library = notrios_abi_version();
    printf("  header ABI major %u, library ABI major %u\n", NOTRIOS_ABI_VERSION, library);
    expect(library == NOTRIOS_ABI_VERSION, "the majors match, so the calls below mean what the header says");
    if (library != NOTRIOS_ABI_VERSION) {
        /* This is the whole point of the check: stop here rather than call on. */
        fprintf(stderr, "refusing to use a library of another ABI major\n");
        return 1;
    }

    uint64_t capabilities = notrios_capabilities();
    uint64_t understood = capabilities & KNOWN_CAPABILITIES;
    uint64_t newer = capabilities & ~(uint64_t)KNOWN_CAPABILITIES;
    printf("  capabilities 0x%llx: 0x%llx understood, 0x%llx newer than this host\n",
           (unsigned long long)capabilities, (unsigned long long)understood,
           (unsigned long long)newer);
    expect(understood == KNOWN_CAPABILITIES,
           "every capability this host needs is present");
    /* Deliberately not an error: a newer library is allowed to know more. */
    printf("  %-4s unknown capability bits are ignored, not refused\n", "ok");

    uint64_t instance = 0;
    expect(notrios_instance_open(argv[1], strlen(argv[1]), &instance) == OK,
           "a library of the expected major opens");
    notrios_instance_close(instance);

    printf("compatibility: %s\n", failures == 0 ? "ok" : "FAILED");
    return failures == 0 ? 0 : 1;
}
