package net

import (
	connmgr2 "github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
)

func NewConnectMgr(low, high int) (connmgr2.ConnManager, error) {
	mgr, err := connmgr.NewConnManager(low, high)
	if err != nil {
		return nil, err
	}

	return mgr, nil
}
