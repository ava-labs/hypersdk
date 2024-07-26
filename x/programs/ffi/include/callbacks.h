#include "types.h"

int bridge_get_callback(GetStateCallback getFuncPtr, void *dbPtr, char *key, int keyLen) {
    return getFuncPtr(dbPtr, key, keyLen);
}