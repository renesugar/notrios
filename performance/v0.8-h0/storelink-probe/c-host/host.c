#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "libnotrios_store_probe.h"

int main(int argc, char **argv) {
  if (argc != 2) {
    fprintf(stderr, "usage: host DATABASE\n");
    return 2;
  }
  char version[32];
  size_t required = notrios_store_probe_sqlite_version(version, sizeof(version));
  if (required == 0 || strcmp(version, "3.53.4") != 0) {
    fprintf(stderr, "unexpected SQLite version: %s\n", version);
    return 1;
  }
  if (notrios_store_probe_open(argv[1], strlen(argv[1])) != 0) {
    fputs("schema-v27 store open failed\n", stderr);
    return 1;
  }
  printf("storelink-probe: SQLite %s schema-v27 PASS\n", version);
  return 0;
}
