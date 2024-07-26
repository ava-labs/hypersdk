#include <stdint.h>
#include <stdio.h>

typedef struct {
   // View v; maybe we could use a view here but not sure?
   int state;
} SimpleMutable;

typedef struct {
    char* method;
    uint8_t* params;
    unsigned int paramLength;
    unsigned int maxGas;
} ExecutionRequest;

typedef struct {
    char address[33];
} Address;

typedef struct {
    char id[32];
} ID;

typedef struct {
    Address programAddress;
    Address actorAddress;
    unsigned int height;
    unsigned int timestamp;
} SimulatorContext;

typedef struct {
    int id;
    char* error;
    uint8_t* result;
} Response;

typedef void (*RustCallback)(int num);

