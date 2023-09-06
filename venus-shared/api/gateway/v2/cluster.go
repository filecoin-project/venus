package gateway

import "context"

type ICluster interface {
	Join(ctx context.Context, address string) error        //perm:admin
	MemberInfos(ctx context.Context) ([]MemberInfo, error) //perm:read
}

type MemberInfo struct {
	Name    string
	Address string
	Meta    map[string]string
}
