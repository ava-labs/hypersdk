#include <stdint.h>
#include <stdio.h>

typedef struct {
    unsigned char address[33];
} Address;

typedef struct {
    unsigned char id[32];
} ID;

typedef struct {
    uint8_t* data;
    unsigned int length;
} Bytes;

// Context needed for runtime.Call
typedef struct {
    // address of the program being invoked
    Address program_address;
    // invoker
    Address actor_address;
    // block height
    unsigned int height;
    // block timestamp
    unsigned int timestamp;
    // method being called on program
    const char* method;
    // params borsh serialized as byte vector
    const uint8_t* params;
    unsigned int param_length;
    // max allowed gas during execution
    unsigned int max_gas;
} SimulatorCallContext;

typedef struct {
    int id;
    char* error;
    Bytes result;
} Response;

typedef struct {
    uint8_t* data;
    unsigned int length;
    const char* error;
} BytesWithError;

// ideally this would return an error enum, but couldn't figure out how include enums in headers(cgo giving me compiler errors)
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
    Address program_address;
    ID program_id;
    const char *error;
} CreateProgramResponse;