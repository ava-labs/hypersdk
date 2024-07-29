#include "types.h"

Bytes bridge_get_callback(GetStateCallback getFuncPtr, void *dbPtr, Bytes key) {
    return getFuncPtr(dbPtr, key);
}

Bytes bridge_insert_callback(InsertStateCallback insertFuncPtr, void *dbPtr, Bytes key, Bytes value) {
    return insertFuncPtr(dbPtr, key, value);
}

Bytes bridge_remove_callback(RemoveStateCallback removeFuncPtr, void *dbPtr, Bytes key) {
    return removeFuncPtr(dbPtr, key);
}