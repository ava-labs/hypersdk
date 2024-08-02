/*
* Go cannot call C function pointers directly, so 
* we define wrapper C functions instead.
* 
* Moreover, this needs to be in a seperate header file since per cgo spec
* "Using //export in a file places a restriction on the preamble: ...
* it must not contain any definitions, only declarations."
* https://pkg.go.dev/cmd/cgo#hdr-C_references_to_Go
*/
#include "types.h"

BytesWithError bridge_get_callback(GetStateCallback getFuncPtr, void *dbPtr, Bytes key) {
    return getFuncPtr(dbPtr, key);
}

char *bridge_insert_callback(InsertStateCallback insertFuncPtr, void *dbPtr, Bytes key, Bytes value) {
    return insertFuncPtr(dbPtr, key, value);
}

char *bridge_remove_callback(RemoveStateCallback removeFuncPtr, void *dbPtr, Bytes key) {
    return removeFuncPtr(dbPtr, key);
}