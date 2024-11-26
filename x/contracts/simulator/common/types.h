/*
 *
 * This header file defines a common set of types and structures used as part of the
 * Foreign Function Interface between Go(CGO) in the context
 * of a smart contract simulator.
 *
 */

#include <stdint.h>
#include <stddef.h>

typedef struct {
    unsigned char address[33];
} Address;

typedef struct {
    const uint8_t* data;
    size_t length;
} Bytes;

typedef struct {
    uint8_t pass_all;
    uint64_t units;
} Gas;

typedef Bytes ContractId;

// Bytes with an additional error field
typedef struct {
    Bytes bytes;
    const char* error;
} BytesWithError;

// Context needed to invoke a contract's method
typedef struct {
    // address of the contract being invoked
    Address contract_address;
    // invoker
    Address actor_address;
    // block height
    uint64_t height;
    // block timestamp
    uint64_t timestamp;
    // method being called on contract
    const char* method;
    // params borsh serialized as byte vector
    Bytes params;
    // passed gas during execution
    Gas gas;
} SimulatorCallContext;

// Response from calling a contract
typedef struct {
    char* error;
    Bytes result;
    uint64_t fuel;
} CallContractResponse;

// Response from creating a contract
typedef struct {
    Address contract_address;
    ContractId contract_id;
    const char *error;
} CreateContractResponse;

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
