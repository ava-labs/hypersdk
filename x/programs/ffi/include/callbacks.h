#include "types.h"

Bytes bridge_get_callback(GetStateCallback getFuncPtr, void *dbPtr, Bytes key) {
    return getFuncPtr(dbPtr, key);
}