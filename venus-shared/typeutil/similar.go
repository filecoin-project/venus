package typeutil

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/filecoin-project/go-state-types/cbor"
)

type CodecFlag uint

//go:generate go run golang.org/x/tools/cmd/stringer -type=CodecFlag -trimprefix=Codec
const (
	CodecBinary CodecFlag = 1 << iota
	CodecText
	CodecJSON
	CodecCbor
	_codecLimit
)

type SimilarMode uint

const (
	StructFieldsOrdered SimilarMode = 1 << iota
	StructFieldTagsMatch
	InterfaceAllMethods
	AvoidRecursive
)

var codecs = []struct {
	flag        CodecFlag
	marshaler   reflect.Type
	unmarshaler reflect.Type
}{
	{
		flag:        CodecBinary,
		marshaler:   reflect.TypeOf((*encoding.BinaryMarshaler)(nil)).Elem(),
		unmarshaler: reflect.TypeOf((*encoding.BinaryUnmarshaler)(nil)).Elem(),
	},
	{
		flag:        CodecText,
		marshaler:   reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem(),
		unmarshaler: reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem(),
	},
	{
		flag:        CodecJSON,
		marshaler:   reflect.TypeOf((*json.Marshaler)(nil)).Elem(),
		unmarshaler: reflect.TypeOf((*json.Unmarshaler)(nil)).Elem(),
	},
	{
		flag:        CodecCbor,
		marshaler:   reflect.TypeOf((*cbor.Marshaler)(nil)).Elem(),
		unmarshaler: reflect.TypeOf((*cbor.Unmarshaler)(nil)).Elem(),
	},
}

type similarResult struct {
	similar bool
	reason  *Reason
}

type similarInput struct {
	a         reflect.Type
	b         reflect.Type
	codecFlag CodecFlag
	smode     SimilarMode
}

var similarCache = struct {
	sync.RWMutex
	results map[similarInput]similarResult
}{
	results: make(map[similarInput]similarResult),
}

type SimilarStack = [2]reflect.Type

func Similar(a, b interface{}, codecFlag CodecFlag, smode SimilarMode, stack ...SimilarStack) (bool, *Reason) {
	atyp, ok := a.(reflect.Type)
	if !ok {
		atyp = reflect.TypeOf(a)
	}

	btyp, ok := b.(reflect.Type)
	if !ok {
		btyp = reflect.TypeOf(b)
	}

	if atyp == btyp {
		return true, nil
	}

	sinput := similarInput{
		a:         atyp,
		b:         btyp,
		codecFlag: codecFlag,
		smode:     smode,
	}

	similarCache.RLock()
	res, has := similarCache.results[sinput]
	if !has {
		sinput.a, sinput.b = btyp, atyp
		res, has = similarCache.results[sinput]
	}
	similarCache.RUnlock()

	if has {
		return res.similar, res.reason
	}

	reasonf := makeReasonf(atyp, btyp)
	reasonWrap := makeReasonWrap(atyp, btyp)

	// recursive
	for si := range stack {
		// we assumpt that they are similar
		// but we won't cache the result
		if (stack[si][0] == atyp && stack[si][1] == btyp) || (stack[si][1] == atyp && stack[si][0] == btyp) {
			return smode&AvoidRecursive == 0, reasonf("%w in the stack #%d, now in #%d", ReasonRecursiveCompare, si, len(stack))
		}
	}

	stack = append(stack, [2]reflect.Type{atyp, btyp})

	var yes bool
	var reason *Reason

	defer func() {
		similarCache.Lock()
		similarCache.results[sinput] = similarResult{
			similar: yes,
			reason:  reason,
		}
		similarCache.Unlock()
	}()

	akind := atyp.Kind()
	bkind := btyp.Kind()

	if akind != bkind {
		reason = reasonf("%w: %s != %s", ReasonTypeKinds, akind, bkind)
		return yes, reason
	}

	if codecFlag != 0 {
		for i := range codecs {
			if codecFlag&codecs[i].flag == 0 {
				continue
			}

			aMarImpl := atyp.Implements(codecs[i].marshaler)
			bMarImpl := btyp.Implements(codecs[i].marshaler)
			if aMarImpl != bMarImpl {
				reason = reasonf("%w for codec %s: %v != %v", ReasonCodecMarshalerImplementations, codecs[i].flag, aMarImpl, bMarImpl)
				return yes, reason
			}

			aUMarImpl := atyp.Implements(codecs[i].unmarshaler)
			bUMarImpl := btyp.Implements(codecs[i].unmarshaler)
			if aUMarImpl != bUMarImpl {
				reason = reasonf("%w for codec %s: %v; %v", ReasonCodecUnmarshalerImplementations, codecs[i].flag, aUMarImpl, bUMarImpl)
				return yes, reason
			}
		}
	}

	switch akind {
	case reflect.Bool:
		fallthrough

	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		fallthrough

	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		fallthrough

	case reflect.Float32, reflect.Float64:
		fallthrough

	case reflect.Complex64, reflect.Complex128:
		fallthrough

	case reflect.Uintptr, reflect.UnsafePointer:
		fallthrough

	case reflect.String:
		yes = true

	case reflect.Interface:

	default:
		yes = atyp.ConvertibleTo(btyp)
	}

	if yes {
		return yes, reason
	}

	switch akind {
	case reflect.Array:
		if atyp.Len() != btyp.Len() {
			reason = reasonf("%w: %d != %d", ReasonArrayLength, atyp.Len(), btyp.Len())
			break
		}

		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, smode, stack...)
		if !elemMatch {
			reason = reasonWrap(elemReason, ReasonArrayElement)
			break
		}

		yes = true

	case reflect.Map:
		keyMatch, keyReason := Similar(atyp.Key(), btyp.Key(), codecFlag, smode, stack...)
		if !keyMatch {
			reason = reasonWrap(keyReason, ReasonMapKey)
			break
		}

		valueMatch, valueReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, smode, stack...)
		if !valueMatch {
			reason = reasonWrap(valueReason, ReasonMapValue)
			break
		}

		yes = true

	case reflect.Ptr:
		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, smode, stack...)
		if !elemMatch {
			reason = reasonWrap(elemReason, ReasonPtrElememnt)
			break
		}

		yes = true

	case reflect.Slice:
		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, smode, stack...)
		if !elemMatch {
			reason = reasonWrap(elemReason, ReasonSliceElement)
			break
		}

		yes = true

	case reflect.Struct:
		fieldsMatch, fieldsReason := fieldsSimilar(atyp, btyp, codecFlag, smode, stack...)
		if !fieldsMatch {
			reason = reasonWrap(fieldsReason, ReasonStructField)
			break
		}

		yes = true

	case reflect.Interface:
		methsMatch, methsReason := methodsSimilar(atyp, btyp, codecFlag, smode, stack...)
		if !methsMatch {
			reason = reasonWrap(methsReason, ReasonInterfaceMethod)
			break
		}

		yes = true

	case reflect.Chan:
		adir := atyp.ChanDir()
		bdir := btyp.ChanDir()
		if adir != bdir {
			reason = reasonf("%w: %s != %s", ReasonChanDir, adir, bdir)
			break
		}

		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, smode, stack...)
		if !elemMatch {
			reason = reasonWrap(elemReason, ReasonChanElement)
			break
		}

		yes = true

	case reflect.Func:
		yes, reason = funcSimilar(atyp, btyp, codecFlag, smode, stack...)

	}

	return yes, reason
}

func funcSimilar(atyp, btyp reflect.Type, codecFlag CodecFlag, smode SimilarMode, stack ...SimilarStack) (bool, *Reason) {
	reasonf := makeReasonf(atyp, btyp)
	reasonWrap := makeReasonWrap(atyp, btyp)

	aNumIn := atyp.NumIn()
	bNumIn := btyp.NumIn()
	if aNumIn != bNumIn {
		return false, reasonf("%w: %d != %d", ReasonFuncInNum, aNumIn, bNumIn)
	}

	aNumOut := atyp.NumOut()
	bNumOut := btyp.NumOut()
	if aNumOut != bNumOut {
		return false, reasonf("%w: %d != %d", ReasonFuncOutNum, aNumOut, bNumOut)
	}

	for i := 0; i < aNumIn; i++ {
		inMatch, inReason := Similar(atyp.In(i), btyp.In(i), codecFlag, smode, stack...)
		if !inMatch {
			return false, reasonWrap(inReason, fmt.Errorf("%w: #%d input", ReasonFuncInType, i))
		}
	}

	for i := 0; i < aNumOut; i++ {
		outMatch, outReason := Similar(atyp.Out(i), btyp.Out(i), codecFlag, smode, stack...)
		if !outMatch {
			return false, reasonWrap(outReason, fmt.Errorf("%w: #%d input", ReasonFuncOutType, i))
		}
	}

	return true, nil
}

func fieldsSimilar(a, b reflect.Type, codecFlag CodecFlag, smode SimilarMode, stack ...SimilarStack) (bool, *Reason) {
	reasonf := makeReasonf(a, b)
	reasonWrap := makeReasonWrap(a, b)

	afields := ExportedFields(a)
	bfields := ExportedFields(b)

	if len(afields) != len(bfields) {
		return false, reasonf("%w: %d != %d", ReasonExportedFieldsCount, len(afields), len(bfields))
	}

	if smode&StructFieldsOrdered != 0 {
		for i := range afields {
			if afields[i].Name != bfields[i].Name {
				return false, reasonf("%w: #%d field, %s != %s", ReasonExportedFieldName, i, afields[i].Name, bfields[i].Name)
			}

			if smode&StructFieldTagsMatch != 0 && afields[i].Tag != bfields[i].Tag {
				return false, reasonf("%w: #%d field, %s != %s", ReasonExportedFieldTag, i, afields[i].Tag, bfields[i].Tag)
			}

			yes, reason := Similar(afields[i].Type, bfields[i].Type, codecFlag, smode, stack...)
			if !yes {
				return false, reasonWrap(reason, fmt.Errorf("%w: #%d field named %s", ReasonExportedFieldType, i, afields[i].Name))
			}
		}

		return true, nil
	}

	mfields := map[string]reflect.StructField{}
	for i := range afields {
		mfields[afields[i].Name] = afields[i]
	}

	for i := range bfields {
		f := bfields[i]
		af, has := mfields[f.Name]
		if !has {
			return false, reasonf("%w: named %s", ReasonExportedFieldNotFound, f.Name)
		}

		if smode&StructFieldTagsMatch != 0 && af.Tag != f.Tag {
			return false, reasonf("%w: named field %s, %s != %s", ReasonExportedFieldTag, f.Name, af.Tag, f.Tag)
		}

		yes, reason := Similar(af.Type, f.Type, codecFlag, smode, stack...)
		if !yes {
			return false, reasonWrap(reason, fmt.Errorf("%w: named %s", ReasonExportedFieldType, f.Name))
		}
	}

	return true, nil
}

func methodsSimilar(a, b reflect.Type, codecFlag CodecFlag, smode SimilarMode, stack ...SimilarStack) (bool, *Reason) {
	reasonf := makeReasonf(a, b)
	reasonWrap := makeReasonWrap(a, b)

	ameths := ExportedMethods(a)
	bmeths := ExportedMethods(b)
	if smode&InterfaceAllMethods != 0 {
		ameths = AllMethods(a)
		bmeths = AllMethods(b)
	}

	if len(ameths) != len(bmeths) {
		return false, reasonf("%w: %d != %d", ReasonExportedMethodsCount, len(ameths), len(bmeths))
	}

	for i := range ameths {
		if ameths[i].Name != bmeths[i].Name {
			return false, reasonf("%w: #%d method, %s != %s ", ReasonExportedMethodName, i, ameths[i].Name, bmeths[i].Name)
		}

		yes, reason := Similar(ameths[i].Type, bmeths[i].Type, codecFlag, smode, stack...)
		if !yes {
			return false, reasonWrap(reason, fmt.Errorf("%w: #%d method named %s", ReasonExportedMethodType, i, ameths[i].Name))
		}
	}

	return true, nil
}
