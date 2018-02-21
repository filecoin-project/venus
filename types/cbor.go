package types

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"

	atlas "gx/ipfs/QmSaDQWMxJBMtzQWnGoDppbwSEbHv4aJcD86CMSdszPU4L/refmt/obj/atlas"
)

var (
	// ErrInvalidMessageLength is returned when the message length does not match the expected length.
	ErrInvalidMessageLength = errors.New("invalid message length")
)

// cache registered entries
var entries = []*entry{}

type entry struct {
	Type      interface{}
	Marshal   reflect.Value
	Unmarshal reflect.Value
}

// CborEntryFromStruct creates an atlas entry for the given struct
// such that serialization happens in array format, indexed by the cbor tag.
func CborEntryFromStruct(s interface{}) *atlas.AtlasEntry {
	t := reflect.TypeOf(s)

	if t.Kind() != reflect.Struct {
		panic("CborEntryFromStruct requires a single struct argument")
	}

	errorT := reflect.TypeOf((*error)(nil)).Elem()
	interfaceSlice := reflect.TypeOf([]interface{}{})

	marshalInputT := []reflect.Type{t}
	marshalOutputT := []reflect.Type{interfaceSlice, errorT}
	marshalT := reflect.FuncOf(marshalInputT, marshalOutputT, false)

	unmarshalInputT := []reflect.Type{interfaceSlice}
	unmarshalOutputT := []reflect.Type{t, errorT}
	unmarshalT := reflect.FuncOf(unmarshalInputT, unmarshalOutputT, false)

	// parse tags ahead of time
	tags := map[int]reflect.StructField{}
	highestIndex := 0
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		tag, ok := field.Tag.Lookup("cbor")
		if !ok {
			panic(fmt.Sprintf("missing cbor tag on field: %s", field.Name))
		}
		index, err := strconv.Atoi(tag)
		if err != nil {
			panic(fmt.Sprintf("invalid cbor tag onfield: %s: %s", field.Name, err))
		}
		if index > highestIndex {
			highestIndex = index
		}

		tags[index] = field
	}

	nilVal := reflect.Zero(errorT)

	marshal := reflect.MakeFunc(marshalT, func(args []reflect.Value) []reflect.Value {
		a := args[0]
		result := make([]interface{}, highestIndex+1)
		for index, field := range tags {
			result[index] = a.FieldByName(field.Name).Interface()
		}

		return []reflect.Value{reflect.ValueOf(result), nilVal}
	})

	unmarshal := reflect.MakeFunc(unmarshalT, func(args []reflect.Value) []reflect.Value {
		a := args[0].Interface().([]interface{})
		out := reflect.New(t).Elem()

		if len(a) != highestIndex+1 {
			return []reflect.Value{reflect.Zero(t), reflect.ValueOf(ErrInvalidMessageLength)}
		}
		// TODO: don't panic, return an error if invalid types are detected

		for i, val := range a {
			if val != nil {
				field := tags[i]
				f := out.FieldByName(field.Name)
				f.Set(assimilate(val, field.Type))
			}
		}

		return []reflect.Value{out, nilVal}
	})

	entries = append(entries, &entry{
		Type:      s,
		Marshal:   marshal,
		Unmarshal: unmarshal,
	})

	return atlas.
		BuildEntry(s).
		Transform().
		TransformMarshal(atlas.MakeMarshalTransformFunc(marshal.Interface())).
		TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(unmarshal.Interface())).
		Complete()
}

func assimilate(val interface{}, target reflect.Type) reflect.Value {
	if e := isRegistered(target); e != nil {
		val = assimilateRegistered(val, target, e).Interface()
	}

	switch target.Kind() {
	case reflect.Slice:
		return assimilateSlice(val, target)
	case reflect.Map:
		return assimilateMap(val, target)
	default:
		return assimilatePointer(val, target)
	}
}

func assimilatePointer(val interface{}, target reflect.Type) reflect.Value {
	value := reflect.ValueOf(val)

	if value.Type() == target {
		return value
	}

	if value.Kind() == reflect.Ptr {
		if target.Kind() == reflect.Ptr {
			return convert(value, target)
		}
		return convert(value.Elem(), target)
	}

	if target.Kind() == reflect.Ptr {
		ptr := reflect.New(reflect.TypeOf(val))
		ptr.Elem().Set(value)
		return convert(ptr, target)
	}

	return convert(value, target)
}

func assimilateSlice(val interface{}, target reflect.Type) reflect.Value {
	value := reflect.ValueOf(val)
	list := reflect.MakeSlice(target, value.Len(), value.Cap())
	slType := target.Elem()

	for i := 0; i < list.Len(); i++ {
		v := assimilate(value.Index(i).Interface(), slType)
		list.Index(i).Set(v)
	}
	return list
}

func assimilateMap(val interface{}, target reflect.Type) reflect.Value {
	value := reflect.ValueOf(val)
	keys := value.MapKeys()
	m := reflect.MakeMapWithSize(target, len(keys))
	keyType := target.Key()
	elType := target.Elem()

	for i := 0; i < len(keys); i++ {
		key := keys[i]
		m.SetMapIndex(
			assimilate(key.Interface(), keyType),
			assimilate(value.MapIndex(key).Interface(), elType),
		)
	}
	return m
}

func assimilateRegistered(val interface{}, target reflect.Type, e *entry) reflect.Value {
	res := e.Unmarshal.Call([]reflect.Value{reflect.ValueOf(val)})

	if !res[1].IsNil() {
		panic(res[1].Interface().(error))
	}

	return res[0]
}

func isRegistered(target reflect.Type) *entry {
	for _, e := range entries {
		if target.Kind() == reflect.Ptr && reflect.TypeOf(e.Type) == target.Elem() {
			return e
		}

		if reflect.TypeOf(e.Type) == target {
			return e
		}
	}
	return nil
}

func convert(val reflect.Value, target reflect.Type) reflect.Value {
	valType := val.Type()
	switch target {
	case reflect.TypeOf(&big.Int{}):
		if valType == reflect.TypeOf([]byte{}) || valType == reflect.TypeOf(&[]byte{}) {
			res := big.NewInt(0)
			if val.Kind() == reflect.Ptr {
				res.SetBytes(val.Elem().Interface().([]byte))
			} else {
				res.SetBytes(val.Interface().([]byte))
			}

			return reflect.ValueOf(res)
		}

		return val.Convert(target)
	default:
		return val.Convert(target)
	}
}
