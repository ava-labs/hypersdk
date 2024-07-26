#include "types.h"

int bridge_callback(GetStateCallback cbFunc, void *data) {
    return cbFunc(data);
}