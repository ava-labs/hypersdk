package chain

import (
	"fmt"
	reflect "reflect"
	"sync"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

func marshalValue(p *codec.Packer, v reflect.Value, kind reflect.Kind, typ reflect.Type) ([]byte, error) {
	switch kind {
	case reflect.Uint8:
		p.PackByte(byte(v.Uint()))
	case reflect.Bool:
		if v.Bool() {
			p.PackByte(1)
		} else {
			p.PackByte(0)
		}
	default:
		return nil, fmt.Errorf("unsupported field type: %v", kind)
	}

	return p.Bytes(), nil
}

func UnmarshalAction(data []byte, item interface{}) error {
	r := codec.NewReader(data, len(data))
	v := reflect.ValueOf(item).Elem()
	t := v.Type()

	info := getTypeInfo(t)

	for _, fi := range info {
		field := v.Field(fi.index)
		err := unmarshalValue(r, field, fi.kind, fi.typ)
		if err != nil {
			return err
		}
	}

	return nil
}

func unmarshalValue(r *codec.Packer, v reflect.Value, kind reflect.Kind, typ reflect.Type) error {
	switch kind {
	case reflect.Uint8:
		v.SetUint(uint64(r.UnpackByte()))
	case reflect.Bool:
		b := r.UnpackByte()
		v.SetBool(b != 0)
	default:
		return fmt.Errorf("unsupported field type: %v", kind)
	}

	if r.Err() != nil {
		return r.Err()
	}

	return nil
}

type fieldInfo struct {
	index int
	kind  reflect.Kind
	typ   reflect.Type
}

var (
	typeInfoCache = make(map[reflect.Type][]fieldInfo)
	cacheMutex    sync.RWMutex
)

func getTypeInfo(t reflect.Type) []fieldInfo {
	cacheMutex.RLock()
	info, ok := typeInfoCache[t]
	cacheMutex.RUnlock()
	if ok {
		return info
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	info = make([]fieldInfo, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		info[i] = fieldInfo{
			index: i,
			kind:  field.Type.Kind(),
			typ:   field.Type,
		}
	}
	typeInfoCache[t] = info
	return info
}

func MarshalAction(item interface{}) ([]byte, error) {
	p := codec.NewWriter(0, consts.NetworkSizeLimit) // FIXME: size
	v := reflect.ValueOf(item)
	t := v.Type()

	info := getTypeInfo(t)

	for _, fi := range info {
		field := v.Field(fi.index)
		_, err := marshalValue(p, field, fi.kind, fi.typ)
		if err != nil {
			return nil, err
		}
	}

	return p.Bytes(), nil
}
