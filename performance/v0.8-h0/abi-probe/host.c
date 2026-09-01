#define _POSIX_C_SOURCE 200809L
#include "notrios_abi.h"
#include <assert.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <pthread.h>
#include <time.h>

static void expect(int got, int want) { if (got != want) { fprintf(stderr, "status %d, want %d\n", got, want); _Exit(1); } }
static void wait_ms(long ms) { struct timespec ts={ms/1000,(ms%1000)*1000000L}; nanosleep(&ts,0); }
struct close_args { uint64_t instance; int status; };
static void *close_thread(void *arg) { struct close_args *a=arg; a->status=notrios_instance_close(a->instance); return 0; }
int main(void) {
  assert(notrios_abi_version() == 1);
  assert(notrios_capabilities() == 0x1f);
  uint64_t a=0,b=0,c=0; expect(notrios_instance_open("default",7,&a),0); expect(notrios_instance_open("other",5,&b),0); assert(a && b && a!=b);
  unsigned char borrowed[]="{\"request_id\":1}"; uint64_t call=0; expect(notrios_call_start(a,borrowed,strlen((char*)borrowed),&call),0); borrowed[0]='X';
  unsigned char *out=NULL; size_t n=0; int st=notrios_call_poll(a,call,&out,&n); assert(st==13 || st==0); wait_ms(20); expect(notrios_call_poll(a,call,&out,&n),0); assert(n>0 && out[0]=='{'); expect(notrios_buffer_release(b,out,n),3); expect(notrios_buffer_release(a,out,n),0); expect(notrios_buffer_release(a,out,n),4);
  uint64_t s=0; const unsigned char bytes[]="abcdef";
  expect(notrios_call_start(a,bytes,6,NULL),1); expect(notrios_stream_open(a,bytes,6,NULL),1); expect(notrios_call_start(a,bytes,1048577,&call),10); expect(notrios_stream_open(a,bytes,1048577,&s),10);
  expect(notrios_event_poll(a,&out,&n),0); expect(notrios_buffer_release(a,out,n),0); expect(notrios_event_poll(a,&out,&n),13);
  expect(notrios_stream_open(a,bytes,6,&s),0); expect(notrios_stream_read(a,s,2,&out,&n),0); assert(n==2&&out[0]=='a'); expect(notrios_buffer_release(a,out,n),0); expect(notrios_stream_read(a,s,99,&out,&n),0); assert(n==4&&out[0]=='c'); expect(notrios_buffer_release(a,out,n),0); expect(notrios_stream_read(a,s,1,&out,&n),12); expect(notrios_stream_close(a,s),0); expect(notrios_stream_close(a,s),4);
  uint64_t cancelled=0; expect(notrios_call_start(a,bytes,6,&cancelled),0); expect(notrios_call_cancel(a,cancelled),0); wait_ms(20); expect(notrios_call_poll(a,cancelled,&out,&n),11);
  expect(notrios_call_start(a,bytes,6,&c),0); expect(notrios_call_poll(b,c,&out,&n),4);
  struct close_args ca={a, -1}; pthread_t tid; expect(pthread_create(&tid,0,close_thread,&ca),0); wait_ms(2); expect(notrios_call_start(a,bytes,6,&c),14); expect(notrios_stream_open(a,bytes,6,&s),14); expect(pthread_join(tid,0),0); expect(ca.status,0); expect(notrios_call_start(a,bytes,6,&c),4); expect(notrios_instance_close(a),0); expect(notrios_instance_close(b),0); expect(notrios_instance_close(0),3);
  puts("abi-probe host: PASS"); return 0;
}
