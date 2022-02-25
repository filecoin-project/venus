package api

import (
	"fmt"
	"testing"
)

func TestAPIInfo_DialArgs(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{
			"common",
			"http://192.168.5.61:3453",
			"http://192.168.5.61:3453/rpc/v0",
			false,
		},
		{
			"wss",
			"/ip4/192.168.5.61/tcp/3453/wss",
			"wss://192.168.5.61:3453/rpc/v0",
			false,
		},
		{
			"ws",
			"/ip4/192.168.5.61/tcp/3453/ws",
			"ws://192.168.5.61:3453/rpc/v0",
			false,
		},
		{
			"http",
			"/ip4/192.168.5.61/tcp/34531/http",
			"http://192.168.5.61:34531/rpc/v0",
			false,
		},
		{
			"https",
			"/ip4/192.168.5.61/tcp/34531/https",
			"https://192.168.5.61:34531/rpc/v0",
			false,
		},
		{
			"default to ws ",
			"/ip4/192.168.5.61/tcp/34532",
			"ws://192.168.5.61:34532/rpc/v0",
			false,
		},

		{
			"version",
			"/ip4/192.168.5.61/tcp/34532/version/v1",
			"ws://192.168.5.61:34532/rpc/v1",
			false,
		},
		{
			"version",
			"/ip4/192.168.5.61/tcp/34532/version/v0",
			"ws://192.168.5.61:34532/rpc/v0",
			false,
		},
		{
			"error version",
			"/ip4/192.168.5.61/tcp/34532/version/1v",
			"/ip4/192.168.5.61/tcp/34532/version/1v/rpc/v0",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "error version" {
				fmt.Println()
			}
			a := APIInfo{
				Addr: tt.addr,
			}

			got, err := a.DialArgs("v0")
			if (err != nil) != tt.wantErr {
				t.Errorf("DialArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DialArgs() got = %v, want %v", got, tt.want)
			}
		})
	}
}
