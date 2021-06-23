package funcrule

import (
	"context"
	"reflect"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"golang.org/x/xerrors"
)

type MethodName = string

// Rule [perm:admin,ignore:true]
// Used by Client to generate rule comments
type Rule struct {
	Perm   auth.Permission
	Ignore bool
}

var AllPermissions = []auth.Permission{"read", "write", "sign", "admin"}
var defaultPerms = []auth.Permission{"read"}

func defaultRule() *Rule {
	return &Rule{
		Perm: "read",
	}
}

//permissionVerify the scheduler between API and internal business
func PermissionProxy(in interface{}, out interface{}) {
	ra := reflect.ValueOf(in)
	rint := reflect.ValueOf(out).Elem()
	for i := 0; i < ra.NumMethod(); i++ {
		methodName := ra.Type().Method(i).Name
		field, exists := rint.Type().FieldByName(methodName)
		if !exists {
			//log.Printf("exclude method %s from fullNode", methodName)
			continue
		}

		requiredPerm := field.Tag.Get("perm")
		if requiredPerm == "" {
			panic("missing 'perm' tag on " + field.Name) // ok
		}
		curule := defaultRule()
		curule.Perm = requiredPerm

		fn := ra.Method(i)
		rint.FieldByName(methodName).Set(reflect.MakeFunc(field.Type, func(args []reflect.Value) (results []reflect.Value) {
			ctx := args[0].Interface().(context.Context)
			errNum := 0
			if !auth.HasPerm(ctx, defaultPerms, curule.Perm) {
				errNum++
				goto ABORT
			}
			return fn.Call(args)
		ABORT:
			err := xerrors.Errorf("missing permission to invoke '%s'", methodName)
			if errNum&1 == 1 {
				err = xerrors.Errorf("%s  (need '%s')", err, curule.Perm)
			}
			rerr := reflect.ValueOf(&err).Elem()
			if fn.Type().NumOut() == 2 {
				return []reflect.Value{
					reflect.Zero(fn.Type().Out(0)),
					rerr,
				}
			}
			return []reflect.Value{rerr}
		}))
	}
}
