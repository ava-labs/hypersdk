package codec

import (
	"fmt"
	"math"
	reflect "reflect"
	"sync"
)

func marshalValue(p *Packer, v reflect.Value, kind reflect.Kind, typ reflect.Type) error {
	switch kind {
	case reflect.Struct:
		info := getTypeInfo(typ)
		for _, fi := range info {
			field := v.Field(fi.index)
			err := marshalValue(p, field, fi.kind, fi.typ)
			if err != nil {
				return err
			}
		}
	case reflect.Slice:
		// All arrays are packed with uint16 length (up to 65535 elements), but bytestrings are packed with uint32 length (up to 4294967295 bytes)
		if typ.Elem().Kind() == reflect.Uint8 {
			p.PackBytes(v.Bytes())
		} else {
			if v.Len() > math.MaxUint16 {
				return fmt.Errorf("array length exceeds maximum allowed")
			}
			p.PackShort(uint16(v.Len()))
			for i := 0; i < v.Len(); i++ {
				err := marshalValue(p, v.Index(i), typ.Elem().Kind(), typ.Elem())
				if err != nil {
					return err
				}
			}
		}
	case reflect.Map:
		if v.Len() > math.MaxUint16 {
			return fmt.Errorf("map length exceeds maximum allowed")
		}
		p.PackShort(uint16(v.Len()))
		for _, key := range v.MapKeys() {
			err := marshalValue(p, key, typ.Key().Kind(), typ.Key())
			if err != nil {
				return err
			}
			err = marshalValue(p, v.MapIndex(key), typ.Elem().Kind(), typ.Elem())
			if err != nil {
				return err
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
		if len(v.String()) > math.MaxUint16 {
			return fmt.Errorf("string length exceeds maximum allowed")
		}
		p.PackString(v.String())
	case reflect.Bool:
		p.PackBool(v.Bool())
	default:
		if typ == reflect.TypeOf(Address{}) {
			if v.Interface().(Address) == EmptyAddress {
				return fmt.Errorf("packer does not support empty addresses")
			}
			addr := v.Interface().(Address)
			p.PackAddress(addr)
		} else {
			return fmt.Errorf("unsupported field type: %v", kind)
		}
	}

	return nil
}

func AutoUnmarshalStruct(p *Packer, item interface{}) error {
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

func unmarshalValue(p *Packer, v reflect.Value, kind reflect.Kind, typ reflect.Type) error {
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
			var b []byte
			p.UnpackBytes(-1, false, &b)
			v.SetBytes(b)
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
		length := int(p.UnpackShort())
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
		v.SetInt(int64(p.UnpackInt(false)))
	case reflect.Int64:
		v.SetInt(int64(p.UnpackLong()))
	case reflect.Uint:
		v.SetUint(uint64(p.UnpackLong()))
	case reflect.Uint8:
		v.SetUint(uint64(p.UnpackByte()))
	case reflect.Uint16:
		v.SetUint(uint64(p.UnpackShort()))
	case reflect.Uint32:
		v.SetUint(uint64(p.UnpackInt(false)))
	case reflect.Uint64:
		v.SetUint(uint64(p.UnpackLong()))
	case reflect.String:
		v.SetString(p.UnpackString(false))
	case reflect.Bool:
		v.SetBool(p.UnpackBool())
	default:
		if typ == reflect.TypeOf(Address{}) {
			var addr Address
			p.UnpackAddress(&addr)
			v.Set(reflect.ValueOf(addr))
		} else {
			return fmt.Errorf("unsupported field type: %v", kind)
		}
	}

	if p.Err() != nil {
		return p.Err()
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

func AutoMarshalStruct(item interface{}, p *Packer) {
	v := reflect.ValueOf(item)
	t := v.Type()

	// Handle pointer to struct
	if t.Kind() == reflect.Ptr {
		if v.IsNil() {
			p.addErr(fmt.Errorf("cannot marshal nil pointer"))
			return
		}
		v = v.Elem()
		t = v.Type()
	}

	// Ensure we're dealing with a struct
	if t.Kind() != reflect.Struct {
		p.addErr(fmt.Errorf("AutoMarshalStruct expects a struct or pointer to struct, got %v", t.Kind()))
		return
	}

	info := getTypeInfo(t)

	for _, fi := range info {
		field := v.Field(fi.index)
		err := marshalValue(p, field, fi.kind, fi.typ)
		if err != nil {
			p.addErr(err)
		}
	}
}
