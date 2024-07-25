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
	case reflect.Struct:
		info := getTypeInfo(typ)
		for _, fi := range info {
			field := v.Field(fi.index)
			_, err := marshalValue(p, field, fi.kind, fi.typ)
			if err != nil {
				return nil, err
			}
		}
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			if v.IsNil() || v.Len() == 0 {
				p.PackBytes(nil)
			} else {
				p.PackBytes(v.Bytes())
			}
		} else {
			p.PackInt(v.Len())
			for i := 0; i < v.Len(); i++ {
				_, err := marshalValue(p, v.Index(i), typ.Elem().Kind(), typ.Elem())
				if err != nil {
					return nil, err
				}
			}
		}
	case reflect.Map:
		p.PackInt(v.Len())
		for _, key := range v.MapKeys() {
			_, err := marshalValue(p, key, typ.Key().Kind(), typ.Key())
			if err != nil {
				return nil, err
			}
			_, err = marshalValue(p, v.MapIndex(key), typ.Elem().Kind(), typ.Elem())
			if err != nil {
				return nil, err
			}
		}
	case reflect.Int:
		p.PackInt64(int64(v.Int()))
	case reflect.Int8:
		p.PackByte(byte(v.Int()))
	case reflect.Int16, reflect.Int32:
		p.PackInt(int(v.Int()))
	case reflect.Int64:
		p.PackInt64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		p.PackInt(int(v.Uint()))
	case reflect.Uint64:
		p.PackUint64(v.Uint())
	case reflect.String:
		p.PackString(v.String())
	case reflect.Bool:
		if v.Bool() {
			p.PackByte(1)
		} else {
			p.PackByte(0)
		}
	default:
		if typ == reflect.TypeOf(codec.Address{}) {
			if v.Interface().(codec.Address) == codec.EmptyAddress {
				return nil, fmt.Errorf("packer does not support empty addresses")
			}
			p.PackAddress(v.Interface().(codec.Address))
		} else {
			return nil, fmt.Errorf("unsupported field type: %v", kind)
		}
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
	case reflect.Struct:
		info := getTypeInfo(typ)
		for _, fi := range info {
			field := v.Field(fi.index)
			err := unmarshalValue(r, field, fi.kind, fi.typ)
			if err != nil {
				return err
			}
		}
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			var bytes []byte
			r.UnpackBytes(-1, false, &bytes)
			v.SetBytes(bytes)
		} else {
			length := r.UnpackInt(false)
			slice := reflect.MakeSlice(typ, length, length)
			for i := 0; i < length; i++ {
				err := unmarshalValue(r, slice.Index(i), typ.Elem().Kind(), typ.Elem())
				if err != nil {
					return err
				}
			}
			v.Set(slice)
		}
	case reflect.Map:
		length := r.UnpackInt(false)
		m := reflect.MakeMap(typ)
		for i := 0; i < length; i++ {
			key := reflect.New(typ.Key()).Elem()
			err := unmarshalValue(r, key, typ.Key().Kind(), typ.Key())
			if err != nil {
				return err
			}
			value := reflect.New(typ.Elem()).Elem()
			err = unmarshalValue(r, value, typ.Elem().Kind(), typ.Elem())
			if err != nil {
				return err
			}
			m.SetMapIndex(key, value)
		}
		v.Set(m)
	case reflect.Int:
		v.SetInt(r.UnpackInt64(false))
	case reflect.Int8:
		v.SetInt(int64(r.UnpackByte()))
	case reflect.Int16, reflect.Int32:
		v.SetInt(int64(r.UnpackInt(false)))
	case reflect.Int64:
		v.SetInt(r.UnpackInt64(false))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		v.SetUint(uint64(r.UnpackInt(false)))
	case reflect.Uint64:
		v.SetUint(r.UnpackUint64(false))
	case reflect.String:
		v.SetString(r.UnpackString(false))
	case reflect.Bool:
		b := r.UnpackByte()
		v.SetBool(b != 0)
	default:
		if typ == reflect.TypeOf(codec.Address{}) {
			var addr codec.Address
			r.UnpackAddress(&addr)
			v.Set(reflect.ValueOf(addr))
		} else {
			return fmt.Errorf("unsupported field type: %v", kind)
		}
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
