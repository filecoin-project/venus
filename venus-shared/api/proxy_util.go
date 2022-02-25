package api

import "reflect"

var _internalField = "Internal"

// GetInternalStructs extracts all pointers to 'Internal' sub-structs from the provided pointer to a proxy struct
func GetInternalStructs(in interface{}) []interface{} {
	return getInternalStructs(reflect.ValueOf(in).Elem())
}

func getInternalStructs(rv reflect.Value) []interface{} {
	var out []interface{}

	for i := 0; i < rv.NumField(); i++ {
		filedValue := rv.Field(i)
		filedType := rv.Type().Field(i)
		if filedType.Name == _internalField {
			ii := filedValue.Addr().Interface()
			out = append(out, ii)
			continue
		}

		sub := getInternalStructs(rv.Field(i))

		out = append(out, sub...)
	}

	return out
}
