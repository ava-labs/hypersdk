package bridge

/*
#cgo CFLAGS: -I../ffi/include
#include "callbacks.h"
#include <stdlib.h>
*/
import "C"
import "unsafe"

type GetStateCallbackType = C.GetStateCallback

// func BridgeCallback(cbFuncPtr unsafe.Pointer, cbData unsafe.Pointer) int {
// 	cbFunc := C.GetStateCallback(cbFuncPtr)
// 	return int(C.bridge_callback(cbFunc, cbData))
// }


// In C, a function argument written as a fixed size array actually requires a 
// pointer to the first element of the array. C compilers are aware of this calling 
// convention and adjust the call accordingly, but Go cannot. In Go, you must pass
// the pointer to the first element explicitly: C.f(&C.x[0]).
func GetCallbackWrapper(getFuncPtr GetStateCallbackType, dbPtr unsafe.Pointer, key []byte) int {
	bytesPtr := C.CBytes(key)
	defer C.free(bytesPtr)

	bytesStruct := C.Bytes{
		data: (*C.uint8_t)(bytesPtr),
		length: C.uint(len(key)),
	}
	return int(C.bridge_get_callback(getFuncPtr, dbPtr, bytesStruct))
}