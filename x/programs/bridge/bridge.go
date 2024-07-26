package bridge

/*
  typedef void (*RustCallback)(void *data);

  void bridge_callback(RustCallback cbFunc, void *data) {
    return cbFunc(data);
  }
*/
import "C"
import "unsafe"

func BridgeCallback(cbFuncPtr unsafe.Pointer, cbData unsafe.Pointer) {
	cbFunc := C.RustCallback(cbFuncPtr)
	C.bridge_callback(cbFunc, cbData)
}