#include "types.h"

Bytes bridge_get_callback(GetStateCallback getFuncPtr, void *dbPtr, Bytes key) {
    return getFuncPtr(dbPtr, key);
}

// ideally this would return an error enum, but couldn't figure out how include enums in headers(cgo giving me compiler errors)
char *bridge_insert_callback(InsertStateCallback insertFuncPtr, void *dbPtr, Bytes key, Bytes value) {
    return insertFuncPtr(dbPtr, key, value);
}

char *bridge_remove_callback(RemoveStateCallback removeFuncPtr, void *dbPtr, Bytes key) {
    return removeFuncPtr(dbPtr, key);
}