#include <stdint.h>

typedef struct {
    unsigned char address[33];
} Address;

typedef struct {
    unsigned char id[32];
} ID;

typedef struct {
    const uint8_t* data;
    unsigned int length;
} Bytes;

// Bytes with an additional error field
typedef struct {
    Bytes bytes;
    const char* error;
} BytesWithError;

// Context needed to invoke a program's method
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
    Bytes params;
    // max allowed gas during execution
    unsigned int max_gas;
} SimulatorCallContext;

// Response from calling a program
typedef struct {
    char* error;
    Bytes result;
    unsigned int fuel;
} CallProgramResponse;

// Response from creating a program
typedef struct {
    Address program_address;
    ID program_id;
    const char *error;
} CreateProgramResponse;

// Callback functions for the mutable interface
typedef BytesWithError (*GetStateCallback)(void *data, Bytes key);
typedef char *(*InsertStateCallback)(void *data, Bytes key, Bytes value);
typedef char *(*RemoveStateCallback)(void *data, Bytes key);

typedef struct {
    // this is a pointer to the state passed in from rust
    // it points to the state object on the rust side of the simulator
    void *stateObj;
    // this is ptr to the get function
    GetStateCallback get_value_callback;
    // this is ptr to the insert function
    InsertStateCallback insert_callback;
	// this is a ptr to the delete function
    RemoveStateCallback remove_callback;
} Mutable;
