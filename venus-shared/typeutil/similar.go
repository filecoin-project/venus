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
	reason  string
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

func Similar(a, b interface{}, codecFlag CodecFlag, ordered Ordered) (bool, string) {
	atyp, ok := a.(reflect.Type)
	if !ok {
		atyp = reflect.TypeOf(a)
	}

	btyp, ok := b.(reflect.Type)
	if !ok {
		btyp = reflect.TypeOf(b)
	}

	if atyp == btyp {
		return true, ""
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
	var reason string

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
		reason = fmt.Sprintf("kinds not match, %s != %s", akind, bkind)
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
				reason = fmt.Sprintf("codec marshaler implementations not match, a: %v, b: %v", aMarImpl, bMarImpl)
				return yes, reason
			}

			aUMarImpl := atyp.Implements(codecs[i].unmarshaler)
			bUMarImpl := btyp.Implements(codecs[i].unmarshaler)
			if aUMarImpl != bUMarImpl {
				reason = fmt.Sprintf("codec unmarshaler implementations not match, a: %v, b: %v", aMarImpl, bMarImpl)
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

	case reflect.String:
		yes = true

	case reflect.Array:
		if atyp.Len() != btyp.Len() {
			reason = fmt.Sprintf("arrays with different length: %d != %d", atyp.Len(), btyp.Len())
			break
		}

		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !elemMatch {
			reason = fmt.Sprintf("array element not match: %s", elemReason)
			break
		}

		yes = true

	case reflect.Map:
		keyMatch, keyReason := Similar(atyp.Key(), btyp.Key(), codecFlag, ordered)
		if !keyMatch {
			reason = fmt.Sprintf("map key not match: %s", keyReason)
			break
		}

		valueMatch, valueReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !valueMatch {
			reason = fmt.Sprintf("map value not match: %s", valueReason)
			break
		}

		yes = true

	case reflect.Ptr:
		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !elemMatch {
			reason = fmt.Sprintf("elem of ptr not match: %s", elemReason)
			break
		}

		yes = true

	case reflect.Slice:
		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !elemMatch {
			reason = fmt.Sprintf("slice element not match: %s", elemReason)
			break
		}

		yes = true

	case reflect.Struct:
		fieldsMatch, fieldsReason := fieldsSimilar(atyp, btyp, codecFlag, ordered)
		if !fieldsMatch {
			reason = fmt.Sprintf("exported fields not match: %s", fieldsReason)
			break
		}

		yes = true

	case reflect.Interface:
		methsMatch, methsReason := methodsSimilar(atyp, btyp, codecFlag, ordered)
		if !methsMatch {
			reason = fmt.Sprintf("exported methods not match: %s", methsReason)
			break
		}

		yes = true

	case reflect.Chan:
		adir := atyp.ChanDir()
		bdir := btyp.ChanDir()
		if adir != bdir {
			reason = fmt.Sprintf("chan dir not match, %d != %d", adir, bdir)
			break
		}

		elemMatch, elemReason := Similar(atyp.Elem(), btyp.Elem(), codecFlag, ordered)
		if !elemMatch {
			reason = fmt.Sprintf("chan element not match: %s", elemReason)
			break
		}

		yes = true

	case reflect.Func:
		yes, reason = funcSimilar(atyp, btyp, codecFlag, ordered)

	default:
		reason = fmt.Sprintf("unexpected type kind %s", akind)
	}

	return yes, reason
}

func funcSimilar(atyp, btyp reflect.Type, codecFlag CodecFlag, ordered Ordered) (bool, string) {
	aNumIn := atyp.NumIn()
	bNumIn := btyp.NumIn()
	if aNumIn != bNumIn {
		return false, fmt.Sprintf("num of inputs not match, %d != %d", aNumIn, bNumIn)
	}

	aNumOut := atyp.NumOut()
	bNumOut := btyp.NumOut()
	if aNumOut != bNumOut {
		return false, fmt.Sprintf("num of outputs not match, %d != %d", aNumOut, bNumOut)
	}

	for i := 0; i < aNumIn; i++ {
		inMatch, inReason := Similar(atyp.In(i), btyp.In(i), codecFlag, ordered)
		if !inMatch {
			return false, fmt.Sprintf("#%d input not match: %s", i, inReason)
		}
	}

	for i := 0; i < aNumOut; i++ {
		outMatch, outReason := Similar(atyp.Out(i), btyp.Out(i), codecFlag, ordered)
		if !outMatch {
			return false, fmt.Sprintf("#%d output not match: %s", i, outReason)
		}
	}

	return true, ""
}

func fieldsSimilar(a, b reflect.Type, codecFlag CodecFlag, ordered Ordered) (bool, string) {
	afields, err := ExportedFields(a)
	if err != nil {
		return false, err.Error()
	}

	bfields, err := ExportedFields(b)
	if err != nil {
		return false, err.Error()
	}

	if len(afields) != len(bfields) {
		return false, fmt.Sprintf("fields count not match, %d != %d", len(afields), len(bfields))
	}

	if ordered&StructFieldsOrdered != 0 {
		for i := range afields {
			yes, reason := Similar(afields[i].Type, bfields[i].Type, codecFlag, ordered)
			if !yes {
				return false, fmt.Sprintf("#%d field not match: %s", i, reason)
			}
		}

		return true, ""
	}

	mfields := map[string]reflect.Type{}
	for i := range afields {
		mfields[afields[i].Name] = afields[i].Type
	}

	for i := range bfields {
		f := bfields[i]
		typ, has := mfields[f.Name]
		if !has {
			return false, fmt.Sprintf("named field %s of %s not found", f.Name, b)
		}

		yes, reason := Similar(typ, f.Type, codecFlag, ordered)
		if !yes {
			return false, fmt.Sprintf("named field %s not match: %s", f.Name, reason)
		}
	}

	return true, ""
}

func methodsSimilar(a, b reflect.Type, codecFlag CodecFlag, ordered Ordered) (bool, string) {
	ameths, err := ExportedMethods(a)
	if err != nil {
		return false, err.Error()
	}

	bmeths, err := ExportedMethods(b)
	if err != nil {
		return false, err.Error()
	}

	if len(ameths) != len(bmeths) {
		return false, fmt.Sprintf("methods count not match, %d != %d", len(ameths), len(bmeths))
	}

	for i := range ameths {
		if ameths[i].Name != bmeths[i].Name {
			return false, fmt.Sprintf("#%d method name not match: %s != %s", i, ameths[i].Name, bmeths[i].Name)
		}

		yes, reason := Similar(ameths[i].Type, bmeths[i].Type, codecFlag, ordered)
		if !yes {
			return false, fmt.Sprintf("#%d method not match: %s", i, reason)
		}
	}

	return true, ""
}
