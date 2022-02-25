package typeutil

import (
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

func ExportedMethods(obj interface{}) []reflect.Method {
	typ, ok := obj.(reflect.Type)
	if !ok {
		typ = reflect.TypeOf(obj)
	}

	exportedMethodsCache.RLock()
	methods, ok := exportedMethodsCache.methods[typ]
	exportedMethodsCache.RUnlock()

	if ok {
		return methods
	}

	all := AllMethods(typ)
	methods = make([]reflect.Method, 0, len(all))
	for i := range all {
		method := all[i]
		if !ast.IsExported(method.Name) {
			continue
		}

		methods = append(methods, method)
	}

	exportedMethodsCache.Lock()
	exportedMethodsCache.methods[typ] = methods
	exportedMethodsCache.Unlock()

	return methods
}

var allMethodsCache = struct {
	sync.RWMutex
	methods map[reflect.Type][]reflect.Method
}{
	methods: make(map[reflect.Type][]reflect.Method),
}

func AllMethods(obj interface{}) []reflect.Method {
	typ, ok := obj.(reflect.Type)
	if !ok {
		typ = reflect.TypeOf(obj)
	}

	allMethodsCache.RLock()
	methods, ok := allMethodsCache.methods[typ]
	allMethodsCache.RUnlock()

	if ok {
		return methods
	}

	num := typ.NumMethod()
	methods = make([]reflect.Method, 0, num)
	for i := 0; i < num; i++ {
		method := typ.Method(i)
		methods = append(methods, method)
	}

	allMethodsCache.Lock()
	allMethodsCache.methods[typ] = methods
	allMethodsCache.Unlock()

	return methods
}
