package bridge

/*
#cgo CFLAGS: -I../ffi/include
#include "callbacks.h"
*/
import "C"
import "unsafe"

type GetStateCallbackType = C.GetStateCallback

func BridgeCallback(cbFuncPtr unsafe.Pointer, cbData unsafe.Pointer) int {
	cbFunc := C.GetStateCallback(cbFuncPtr)
	return int(C.bridge_callback(cbFunc, cbData))
}