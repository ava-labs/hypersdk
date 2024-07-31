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
    void* result;
} Response;

typedef struct {
    uint8_t* data;
    unsigned int length;
} Bytes;

typedef struct {
    uint8_t* data;
    unsigned int length;
    const char* error;
} BytesWithError;

typedef BytesWithError (*GetStateCallback)(void *data, Bytes key);
typedef char *(*InsertStateCallback)(void *data, Bytes key, Bytes value);
typedef char *(*RemoveStateCallback)(void *data, Bytes key);

typedef struct {
    void *stateObj;
    GetStateCallback get_value_callback;
    InsertStateCallback insert_callback;
    RemoveStateCallback remove_callback;
    // doesn't need ot be on the bottom on c side, strange why it needs on rust
    void *statePlaceholder;
} Mutable;

typedef struct {
    Address programAddress;
    ID programID;
    const char *err;
} CreateProgramResponse;