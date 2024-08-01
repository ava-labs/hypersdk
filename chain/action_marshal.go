package chain

import (
	"fmt"
	reflect "reflect"
	"sync"

	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

func marshalValue(p *wrappers.Packer, v reflect.Value, kind reflect.Kind, typ reflect.Type) ([]byte, error) {
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
		// All arrays are packed with uint16 length (up to 65535 elements), but bytestrings are packed with uint32 length (up to 4294967295 bytes)
		if typ.Elem().Kind() == reflect.Uint8 {
			p.PackBytes(v.Bytes())
		} else {
			p.PackShort(uint16(v.Len()))
			for i := 0; i < v.Len(); i++ {
				_, err := marshalValue(p, v.Index(i), typ.Elem().Kind(), typ.Elem())
				if err != nil {
					return nil, err
				}
			}
		}
	case reflect.Map:
		p.PackInt(uint32(v.Len()))
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
	case reflect.Int8:
		p.PackByte(byte(v.Int()))
	case reflect.Int16:
		p.PackShort(uint16(v.Int()))
	case reflect.Int32:
		p.PackInt(uint32(v.Int()))
	case reflect.Int64, reflect.Int:
		p.PackLong(uint64(v.Int()))
	case reflect.Uint:
		p.PackLong(v.Uint())
	case reflect.Uint8:
		p.PackByte(byte(v.Uint()))
	case reflect.Uint16:
		p.PackShort(uint16(v.Uint()))
	case reflect.Uint32:
		p.PackInt(uint32(v.Uint()))
	case reflect.Uint64:
		p.PackLong(v.Uint())
	case reflect.String:
		p.PackShort(uint16(v.Len()))
		p.PackFixedBytes([]byte(v.String()))
	case reflect.Bool:
		p.PackBool(v.Bool())
	default:
		if typ == reflect.TypeOf(codec.Address{}) {
			if v.Interface().(codec.Address) == codec.EmptyAddress {
				return nil, fmt.Errorf("packer does not support empty addresses")
			}
			addr := v.Interface().(codec.Address)
			p.PackFixedBytes(addr[:])
		} else {
			return nil, fmt.Errorf("unsupported field type: %v", kind)
		}
	}

	return p.Bytes, nil
}

func UnmarshalAction(data []byte, item interface{}) error {
	p := &wrappers.Packer{
		Bytes: data,
	}
	v := reflect.ValueOf(item).Elem()
	t := v.Type()

	info := getTypeInfo(t)

	for _, fi := range info {
		field := v.Field(fi.index)
		err := unmarshalValue(p, field, fi.kind, fi.typ)
		if err != nil {
			return err
		}
	}

	return nil
}

func unmarshalValue(p *wrappers.Packer, v reflect.Value, kind reflect.Kind, typ reflect.Type) error {
	switch kind {
	case reflect.Struct:
		info := getTypeInfo(typ)
		for _, fi := range info {
			field := v.Field(fi.index)
			if err := unmarshalValue(p, field, fi.kind, fi.typ); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			v.SetBytes(p.UnpackBytes())
		} else {
			length := int(p.UnpackShort())
			slice := reflect.MakeSlice(typ, length, length)
			for i := 0; i < length; i++ {
				if err := unmarshalValue(p, slice.Index(i), typ.Elem().Kind(), typ.Elem()); err != nil {
					return err
				}
			}
			v.Set(slice)
		}
	case reflect.Map:
		length := int(p.UnpackInt())
		m := reflect.MakeMap(typ)
		for i := 0; i < length; i++ {
			key := reflect.New(typ.Key()).Elem()
			if err := unmarshalValue(p, key, typ.Key().Kind(), typ.Key()); err != nil {
				return err
			}
			value := reflect.New(typ.Elem()).Elem()
			if err := unmarshalValue(p, value, typ.Elem().Kind(), typ.Elem()); err != nil {
				return err
			}
			m.SetMapIndex(key, value)
		}
		v.Set(m)
	case reflect.Int:
		v.SetInt(int64(p.UnpackLong()))
	case reflect.Int8:
		v.SetInt(int64(p.UnpackByte()))
	case reflect.Int16:
		v.SetInt(int64(p.UnpackShort()))
	case reflect.Int32:
		v.SetInt(int64(p.UnpackInt()))
	case reflect.Int64:
		v.SetInt(int64(p.UnpackLong()))
	case reflect.Uint:
		v.SetUint(uint64(p.UnpackLong()))
	case reflect.Uint8:
		v.SetUint(uint64(p.UnpackByte()))
	case reflect.Uint16:
		v.SetUint(uint64(p.UnpackShort()))
	case reflect.Uint32:
		v.SetUint(uint64(p.UnpackInt()))
	case reflect.Uint64:
		v.SetUint(uint64(p.UnpackLong()))
	case reflect.String:
		v.SetString(string(p.UnpackFixedBytes(int(p.UnpackShort()))))
	case reflect.Bool:
		v.SetBool(p.UnpackBool())
	default:
		if typ == reflect.TypeOf(codec.Address{}) {
			var addr codec.Address
			copy(addr[:], p.UnpackFixedBytes(len(addr)))
			v.Set(reflect.ValueOf(addr))
		} else {
			return fmt.Errorf("unsupported field type: %v", kind)
		}
	}

	if p.Errs.Err != nil {
		return p.Errs.Err
	}

	return nil
}

type fieldInfo struct {
	index    int
	kind     reflect.Kind
	typ      reflect.Type
	exported bool
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

	var exportedFields []fieldInfo
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.IsExported() {
			exportedFields = append(exportedFields, fieldInfo{
				index: i,
				kind:  field.Type.Kind(),
				typ:   field.Type,
			})
		}
	}
	typeInfoCache[t] = exportedFields
	return exportedFields
}

func MarshalAction(item interface{}) ([]byte, error) {
	p := &wrappers.Packer{MaxSize: consts.NetworkSizeLimit}
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

	return p.Bytes, nil
}
