package permission

import (
	"context"
	"fmt"
	"reflect"

	"github.com/filecoin-project/venus/venus-shared/api"

	"github.com/filecoin-project/go-jsonrpc/auth"
)

type permission = auth.Permission

const (
	// When changing these, update docs/API.md too

	PermRead  permission = "read" // default
	PermWrite permission = "write"
	PermSign  permission = "sign"  // Use wallet keys for signing
	PermAdmin permission = "admin" // Manage permissions

)

var AllPermissions = []auth.Permission{PermRead, PermWrite, PermSign, PermAdmin}
var DefaultPerms = []auth.Permission{PermRead}

// PermissionProxy the scheduler between API and internal business
// nolint
func PermissionProxy(in interface{}, out interface{}) {
	ra := reflect.ValueOf(in)
	outs := api.GetInternalStructs(out)
	for _, out := range outs {
		rint := reflect.ValueOf(out).Elem()
		for i := 0; i < ra.NumMethod(); i++ {
			methodName := ra.Type().Method(i).Name
			field, exists := rint.Type().FieldByName(methodName)
			if !exists {
				continue
			}

			requiredPerm := field.Tag.Get("perm")
			if requiredPerm == "" {
				panic("missing 'perm' tag on " + field.Name) // ok
			}

			var found bool
			for _, perm := range AllPermissions {
				if perm == requiredPerm {
					found = true
				}
			}
			if !found {
				panic("unknown 'perm' tag on " + field.Name)
			}

			fn := ra.Method(i)
			rint.FieldByName(methodName).Set(reflect.MakeFunc(field.Type, func(args []reflect.Value) (results []reflect.Value) {
				ctx := args[0].Interface().(context.Context)
				errNum := 0
				if !auth.HasPerm(ctx, DefaultPerms, requiredPerm) {
					errNum++
					goto ABORT
				}
				return fn.Call(args)
			ABORT:
				err := fmt.Errorf("missing permission to invoke '%s'", methodName)
				if errNum&1 == 1 {
					err = fmt.Errorf("%s  (need '%s')", err, requiredPerm)
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

}
