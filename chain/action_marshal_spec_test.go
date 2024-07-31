package chain_test

import (
	reflect "reflect"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
)

func TestMarshalLengths(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected int
	}{
		{"Bool", struct{ V bool }{true}, 1},
		{"Int8", struct{ V int8 }{127}, 1},
		{"Int16", struct{ V int16 }{32767}, 2},
		{"Int32", struct{ V int32 }{2147483647}, 4},
		{"Int64", struct{ V int64 }{9223372036854775807}, 8},
		{"Uint8", struct{ V uint8 }{255}, 1},
		{"Uint16", struct{ V uint16 }{65535}, 2},
		{"Uint32", struct{ V uint32 }{4294967295}, 4},
		{"Uint64", struct{ V uint64 }{18446744073709551615}, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := chain.MarshalAction(tt.value)
			if err != nil {
				t.Fatalf("MarshalAction failed: %v", err)
			}

			if len(bytes) != tt.expected {
				t.Errorf("Expected length %d, got %d", tt.expected, len(bytes))
			}

			restored := reflect.New(reflect.TypeOf(tt.value)).Interface()
			err = chain.UnmarshalAction(bytes, restored)
			if err != nil {
				t.Fatalf("UnmarshalAction failed: %v", err)
			}

			if !reflect.DeepEqual(tt.value, reflect.ValueOf(restored).Elem().Interface()) {
				t.Errorf("Value mismatch. Expected %v, got %v", tt.value, reflect.ValueOf(restored).Elem().Interface())
			}
		})
	}
}
