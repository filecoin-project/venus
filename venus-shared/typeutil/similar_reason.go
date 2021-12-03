package typeutil

import (
	"errors"
	"fmt"
	"reflect"
)

func makeReasonf(a, b reflect.Type) func(f string, args ...interface{}) *Reason {
	return func(f string, args ...interface{}) *Reason {
		wrapfn := makeReasonWrap(a, b)
		return wrapfn(nil, fmt.Errorf(f, args...))
	}
}

func makeReasonWrap(a, b reflect.Type) func(nested *Reason, base error) *Reason {
	return func(nested *Reason, base error) *Reason {
		return &Reason{
			TypeA:  a,
			TypeB:  b,
			Base:   base,
			Nested: nested,
		}
	}
}

type Reason struct {
	TypeA  reflect.Type
	TypeB  reflect.Type
	Base   error
	Nested *Reason
}

func (r *Reason) Error() string {
	if r == nil || r.Base == nil {
		return "nil"
	}

	return fmt.Sprintf("{[%s <> %s] base=%s; nested=%s}", r.TypeA, r.TypeB, r.Base, r.Nested)
}

func (r *Reason) Is(target error) bool {
	if r == nil || r.Base == nil {
		return false
	}

	return errors.Is(r.Base, target)
}

func (r *Reason) Unwrap() error {
	if r == nil || r.Nested == nil {
		return nil
	}

	return r.Nested
}

var (
	ReasonTypeKinds                       = fmt.Errorf("type kinds")                        // nolint
	ReasonCodecMarshalerImplementations   = fmt.Errorf("codec marshaler implementations")   // nolint
	ReasonCodecUnmarshalerImplementations = fmt.Errorf("codec unmarshaler implementations") // nolint
	ReasonArrayLength                     = fmt.Errorf("array length")                      // nolint
	ReasonArrayElement                    = fmt.Errorf("array element")                     // nolint
	ReasonMapKey                          = fmt.Errorf("map key")                           // nolint
	ReasonMapValue                        = fmt.Errorf("map value")                         // nolint
	ReasonPtrElememnt                     = fmt.Errorf("pointed type")                      // nolint
	ReasonSliceElement                    = fmt.Errorf("slice element")                     // nolint
	ReasonStructField                     = fmt.Errorf("struct field")                      // nolint
	ReasonInterfaceMethod                 = fmt.Errorf("interface method")                  // nolint
	ReasonChanDir                         = fmt.Errorf("channel direction")                 // nolint
	ReasonChanElement                     = fmt.Errorf("channel element")                   // nolint
	ReasonFuncInNum                       = fmt.Errorf("func in num")                       // nolint
	ReasonFuncOutNum                      = fmt.Errorf("func out num")                      // nolint
	ReasonFuncInType                      = fmt.Errorf("func in type")                      // nolint
	ReasonFuncOutType                     = fmt.Errorf("func out type")                     // nolint
	ReasonExportedFieldsCount             = fmt.Errorf("exported fields count")             // nolint
	ReasonExportedFieldName               = fmt.Errorf("exported field name")               // nolint
	ReasonExportedFieldTag                = fmt.Errorf("exported field tag")                // nolint
	ReasonExportedFieldNotFound           = fmt.Errorf("exported field not found")          // nolint
	ReasonExportedFieldType               = fmt.Errorf("exported field type")               // nolint
	ReasonExportedMethodsCount            = fmt.Errorf("exported methods count")            // nolint
	ReasonExportedMethodName              = fmt.Errorf("exported method name")              // nolint
	ReasonExportedMethodType              = fmt.Errorf("exported method type")              // nolint
	ReasonRecursiveCompare                = fmt.Errorf("recursive compare")                 // nolint
)
