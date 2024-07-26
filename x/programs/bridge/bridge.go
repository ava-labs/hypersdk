package bridge

/*
  typedef void (*RustCallback)(int num);

  void bridge_callback(RustCallback cbFunc) {
    return cbFunc(42);
  }
*/
import "C"
import "unsafe"

func BridgeCallback(cbFuncPtr unsafe.Pointer) {
	cbFunc := C.RustCallback(cbFuncPtr)
	C.bridge_callback(cbFunc)
}