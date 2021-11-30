package typecheck

import (
	"fmt"
	"go/ast"
	"reflect"
	"sync"
)

var exportedMethodsCache = struct {
	sync.RWMutex
	methods map[reflect.Type][]reflect.Method
}{
	methods: make(map[reflect.Type][]reflect.Method),
}

func ExportedMethods(obj interface{}) ([]reflect.Method, error) {
	typ, ok := obj.(reflect.Type)
	if !ok {
		typ = reflect.TypeOf(obj)
	}

	if kind := typ.Kind(); kind != reflect.Struct && kind != reflect.Interface {
		return nil, fmt.Errorf("unexpected type kind %s", kind)
	}

	exportedMethodsCache.RLock()
	methods, ok := exportedMethodsCache.methods[typ]
	exportedMethodsCache.RUnlock()

	if ok {
		return methods, nil
	}

	num := typ.NumMethod()
	methods = make([]reflect.Method, 0, num)
	for i := 0; i < num; i++ {
		method := typ.Method(i)
		if !ast.IsExported(method.Name) {
			continue
		}

		methods = append(methods, method)
	}

	exportedMethodsCache.Lock()
	exportedMethodsCache.methods[typ] = methods
	exportedMethodsCache.Unlock()

	return methods, nil
}
