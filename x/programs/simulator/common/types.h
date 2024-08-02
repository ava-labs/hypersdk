#include <stdint.h>
#include <stdio.h>

typedef struct {
   // View v; maybe we could use a view here but not sure?
   int state;
} SimpleMutable;

typedef struct {
    const char* method;
    const uint8_t* params;
    unsigned int param_length;
    unsigned int max_gas;
} ExecutionRequest;

typedef struct {
    unsigned char address[33];
} Address;

typedef struct {
    unsigned char id[32];
} ID;

typedef struct {
    Address program_address;
    Address actor_address;
    unsigned int height;
    unsigned int timestamp;
} SimulatorCallContext;


typedef struct {
    uint8_t* data;
    unsigned int length;
} Bytes;


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