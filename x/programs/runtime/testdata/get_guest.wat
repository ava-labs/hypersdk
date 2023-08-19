(module $test
  (type (;0;) (func (result i32)))
  (export "get_guest" (func 0))
  (func (;0;) (type 0) (result i32)
    ;; initialize a local i32 variable
    (local i32)
    ;; define as 1
    i32.const 1
    ;; return result
  )
)