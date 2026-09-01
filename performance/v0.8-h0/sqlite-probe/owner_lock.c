#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <string.h>
#include <sys/file.h>
#include <unistd.h>

int main(int argc, char **argv) {
  if (argc != 2) return 2;
  int fd = open(argv[1], O_CREAT|O_RDWR, 0600);
  if (fd < 0) { fprintf(stderr,"open: %s\n",strerror(errno)); return 1; }
  if (flock(fd, LOCK_EX|LOCK_NB) != 0) { fprintf(stderr,"owner refused: %s\n",strerror(errno)); close(fd); return 3; }
  puts("owner acquired"); fflush(stdout); sleep(2); flock(fd, LOCK_UN); close(fd); return 0;
}
