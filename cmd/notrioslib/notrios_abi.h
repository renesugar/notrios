#ifndef NOTRIOS_ABI_H
#define NOTRIOS_ABI_H
#include <stdint.h>
#include <stddef.h>
uint32_t notrios_abi_version(void);
uint64_t notrios_capabilities(void);
int32_t notrios_instance_open(const char*, size_t, uint64_t*);
int32_t notrios_instance_close(uint64_t);
int32_t notrios_call_start(uint64_t,const unsigned char*,size_t,uint64_t*);
int32_t notrios_call_poll(uint64_t,uint64_t,unsigned char**,size_t*);
int32_t notrios_call_cancel(uint64_t,uint64_t);
int32_t notrios_buffer_release(uint64_t,unsigned char*,size_t);
int32_t notrios_event_poll(uint64_t,unsigned char**,size_t*);
int32_t notrios_stream_open(uint64_t,const unsigned char*,size_t,uint64_t*);
int32_t notrios_stream_read(uint64_t,uint64_t,size_t,unsigned char**,size_t*);
int32_t notrios_stream_close(uint64_t,uint64_t);
#endif
