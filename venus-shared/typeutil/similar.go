package typeutil

import (
	"encoding"
	"encoding/json"
	"fmt"
	"math/bits"
	"reflect"
	"sync"

	"github.com/filecoin-project/go-state-types/cbor"
)

func init() {
	if zeroes := bits.TrailingZeros(uint(_CodecLimit)); zeroes != len(codecs) {
		panic(fmt.Errorf("codec count not match, %d != %d", zeroes, len(codecs)))
	}

	for ci := range codecs {
		if zeroes := bits.TrailingZeros(uint(codecs[ci].flag)); zeroes != ci {
			panic(fmt.Errorf("#%d codec's flag is not matched", ci))
		}
	}
}

type CodecFlag uint

const (
	BinaryCodec CodecFlag = 1 << iota
	TextCodec
	JSONCodec
	CborCodec
	_CodecLimit
)

type Ordered uint

const (
	StructFieldsOrdered Ordered = 1 << iota
)

var (
	codecs = []struct {
		flag        CodecFlag
		marshaler   reflect.Type
		unmarshaler reflect.Type
	}{
		{
			flag:        BinaryCodec,
			marshaler:   reflect.TypeOf((*encoding.BinaryMarshaler)(nil)).Elem(),
			unmarshaler: reflect.TypeOf((*encoding.BinaryUnmarshaler)(nil)).Elem(),
		},
		{
			flag:        TextCodec,
			marshaler:   reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem(),
			unmarshaler: reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem(),
		},
		{
			flag:        JSONCodec,
			marshaler:   reflect.TypeOf((*json.Marshaler)(nil)).Elem(),
			unmarshaler: reflect.TypeOf((*json.Unmarshaler)(nil)).Elem(),
		},
		{
			flag:        CborCodec,
			marshaler:   reflect.TypeOf((*cbor.Marshaler)(nil)).Elem(),
			unmarshaler: reflect.TypeOf((*cbor.Unmarshaler)(nil)).Elem(),
		},
	}
)

type similarResult struct {
	similar bool
	reason  *Reason
}

type similarInput struct {
	a         reflect.Type
	b         reflect.Type
	codecFlag CodecFlag
	ordered   Ordered
}

var similarCache = struct {
	sync.RWMutex
	results map[similarInput]similarResult
}{
	results: make(map[similarInput]similarResult),
}

func Similar(a, b interface{}, codecFlag CodecFlag, ordered Ordered) (bool, *Reason) {
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
		ordered:   ordered,
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

	var yes bool
	var reason *Reason

	reasonf := makeReasonf(atyp, btyp)

	reasonWrap := makeReasonWrap(atyp, btyp)

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
				reason = reasonf("%w for codec %d: %v != %v", ReasonCodecMarshalerImplementations, codecs[i].flag, aMarImpl, bMarImpl)
				return yes, reason
			}

			aUMarImpl := atyp.Implements(codecs[i].unmarshaler)
			bUMarImpl := btyp.Implements(codecs[i].unmarshaler)
			if aUMarImpl != bUMarImpl {
				reason = reasonf("%w for codec %d: %v; %v", ReasonCodecUnmarshalerImplementations, codecs[i].flag, aUMarImpl, bUMarImpl)
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

	case reflect.Array:
		if atyp.Len() != btyp.Len() {
			reason = reasonf("%w: %d != %d", ReasonArrayLength, atyp.Len(), btyp.Len())
			break
		}

		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !elemMatch {
			reason = reasonWrap(elemReason, ReasonArrayElement)
			break
		}

		yes = true

	case reflect.Map:
		keyMatch, keyReason := Similar(atyp.Key(), btyp.Key(), codecFlag, ordered)
		if !keyMatch {
			reason = reasonWrap(keyReason, ReasonMapKey)
			break
		}

		valueMatch, valueReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !valueMatch {
			reason = reasonWrap(valueReason, ReasonMapValue)
			break
		}

		yes = true

	case reflect.Ptr:
		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !elemMatch {
			reason = reasonWrap(elemReason, ReasonPtrElememnt)
			break
		}

		yes = true

	case reflect.Slice:
		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !elemMatch {
			reason = reasonWrap(elemReason, ReasonSliceElement)
			break
		}

		yes = true

	case reflect.Struct:
		fieldsMatch, fieldsReason := fieldsSimilar(atyp, btyp, codecFlag, ordered)
		if !fieldsMatch {
			reason = reasonWrap(fieldsReason, ReasonStructField)
			break
		}

		yes = true

	case reflect.Interface:
		methsMatch, methsReason := methodsSimilar(atyp, btyp, codecFlag, ordered)
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

		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !elemMatch {
			reason = reasonWrap(elemReason, ReasonChanElement)
			break
		}

		yes = true

	case reflect.Func:
		yes, reason = funcSimilar(atyp, btyp, codecFlag, ordered)

	}

	return yes, reason
}

func funcSimilar(atyp, btyp reflect.Type, codecFlag CodecFlag, ordered Ordered) (bool, *Reason) {
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
		inMatch, inReason := Similar(atyp.In(i), btyp.In(i), codecFlag, ordered)
		if !inMatch {
			return false, reasonWrap(inReason, fmt.Errorf("%w: #%d input", ReasonFuncInType, i))
		}
	}

	for i := 0; i < aNumOut; i++ {
		outMatch, outReason := Similar(atyp.Out(i), btyp.Out(i), codecFlag, ordered)
		if !outMatch {
			return false, reasonWrap(outReason, fmt.Errorf("%w: #%d input", ReasonFuncOutType, i))
		}
	}

	return true, nil
}

func fieldsSimilar(a, b reflect.Type, codecFlag CodecFlag, ordered Ordered) (bool, *Reason) {
	reasonf := makeReasonf(a, b)
	reasonWrap := makeReasonWrap(a, b)

	afields := ExportedFields(a)

	bfields := ExportedFields(b)

	if len(afields) != len(bfields) {
		return false, reasonf("%w: %d != %d", ReasonExportedFieldsCount, len(afields), len(bfields))
	}

	if ordered&StructFieldsOrdered != 0 {
		for i := range afields {
			if afields[i].Name != bfields[i].Name {
				return false, reasonf("%w: #%d field, %s != %s", ReasonExportedFieldName, i, afields[i].Name, bfields[i].Name)
			}

			yes, reason := Similar(afields[i].Type, bfields[i].Type, codecFlag, ordered)
			if !yes {
				return false, reasonWrap(reason, fmt.Errorf("%w: #%d field named %s", ReasonExportedFieldType, i, afields[i].Name))
			}
		}

		return true, nil
	}

	mfields := map[string]reflect.Type{}
	for i := range afields {
		mfields[afields[i].Name] = afields[i].Type
	}

	for i := range bfields {
		f := bfields[i]
		typ, has := mfields[f.Name]
		if !has {
			return false, reasonf("%w: named %s", ReasonExportedFieldNotFound, f.Name)
		}

		yes, reason := Similar(typ, f.Type, codecFlag, ordered)
		if !yes {
			return false, reasonWrap(reason, fmt.Errorf("%w: named %s", ReasonExportedFieldType, f.Name))
		}
	}

	return true, nil
}

func methodsSimilar(a, b reflect.Type, codecFlag CodecFlag, ordered Ordered) (bool, *Reason) {
	reasonf := makeReasonf(a, b)
	reasonWrap := makeReasonWrap(a, b)

	ameths := ExportedMethods(a)
	bmeths := ExportedMethods(b)

	if len(ameths) != len(bmeths) {
		return false, reasonf("%w: %d != %d", ReasonExportedMethodsCount, len(ameths), len(bmeths))
	}

	for i := range ameths {
		if ameths[i].Name != bmeths[i].Name {
			return false, reasonf("%w: #%d method, %s != %s ", ReasonExportedMethodName, i, ameths[i].Name, bmeths[i].Name)
		}

		yes, reason := Similar(ameths[i].Type, bmeths[i].Type, codecFlag, ordered)
		if !yes {
			return false, reasonWrap(reason, fmt.Errorf("%w: #%d method named %s", ReasonExportedMethodType, i, ameths[i].Name))
		}
	}

	return true, nil
}
