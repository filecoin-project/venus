package msgparser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"

	"github.com/filecoin-project/go-address"
	cbor "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v7/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/require"
)

func init() {
	address.CurrentNetwork = address.Mainnet
}

type mockActorGetter struct {
	actors map[string]*types.Actor
}

func (x *mockActorGetter) StateGetActor(ctx context.Context, addr address.Address, _ types.TipSetKey) (*types.Actor, error) {
	actor, isok := x.actors[addr.String()]
	if !isok {
		return nil, fmt.Errorf("address:%s not found", addr.String())
	}
	return actor, nil
}

func newMockActorGetter(t *testing.T) *mockActorGetter {
	return &mockActorGetter{
		actors: map[string]*types.Actor{
			"f01": {Code: builtin.InitActorCodeID},
			"f02": {Code: builtin.RewardActorCodeID},
			"f03": {Code: builtin.CronActorCodeID},
			"f04": {Code: builtin.StoragePowerActorCodeID},
			"f05": {Code: builtin.StorageMarketActorCodeID},
			"f06": {Code: builtin.VerifiedRegistryActorCodeID},
			"f2nbgz5oxgdqjkuvkbtdzwiepisiycnpjyibcvmpy": {Code: builtin.MultisigActorCodeID},
			"f14mj3nt7tlgyditvk6h5p6i36vqebwgfqndjeq6a": {Code: builtin.AccountActorCodeID},
			"f2ttj54ce3xziwij7qnyzbadu3bhxjewriol66tai": {Code: builtin.MultisigActorCodeID},
			"f028027":   {Code: builtin.MultisigActorCodeID},
			"f01481491": {Code: builtin.StorageMinerActorCodeID},
			"f066563":   {Code: builtin.StorageMinerActorCodeID},
			"f0748179":  {Code: builtin.StorageMinerActorCodeID},
			"f0116287":  {Code: builtin.StorageMinerActorCodeID},
			"f033036":   {Code: builtin.StorageMinerActorCodeID},
			"f0807472":  {Code: builtin.StorageMinerActorCodeID},
			"f0392813":  {Code: builtin.StorageMinerActorCodeID},
			"f0441116":  {Code: builtin.StorageMinerActorCodeID},
			"f01111881": {Code: builtin.StorageMinerActorCodeID},
			"f01387570": {Code: builtin.StorageMinerActorCodeID},
			"f0717289":  {Code: builtin.StorageMinerActorCodeID},
			"f01476109": {Code: builtin.StorageMinerActorCodeID},
			"f01451690": {Code: builtin.StorageMinerActorCodeID},
			"f01203636": {Code: builtin.StorageMinerActorCodeID},
			"f01171513": {Code: builtin.StorageMinerActorCodeID},
			"f054464":   {Code: builtin.StorageMinerActorCodeID},
			"f0106363":  {Code: builtin.StorageMinerActorCodeID},
			"f01190965": {Code: builtin.StorageMinerActorCodeID},
			"f02388":    {Code: builtin.StorageMinerActorCodeID},
			"f01203143": {Code: builtin.StorageMinerActorCodeID},
			"f01211558": {Code: builtin.StorageMinerActorCodeID},
			"f01483143": {Code: builtin.StorageMinerActorCodeID},
			"f027083":   {Code: builtin.MultisigActorCodeID},
		},
	}
}

type testCase map[string]interface{}

func (c testCase) name(t *testing.T) string {
	return c.stringField(t, "name")
}

func (c testCase) stringField(t *testing.T, fieldName string) string {
	fs, isok := c[fieldName]
	if !isok || fs == nil {
		return ""
	}
	str, isok := fs.(string)
	if !isok {
		t.Fatalf("'%s' field must be base64 string", fieldName)
	}
	return str
}

func (c testCase) base64Field(t *testing.T, fieldName string) []byte {
	bytes, err := base64.StdEncoding.DecodeString(c.stringField(t, fieldName))
	if err != nil {
		t.Fatalf("decode field(%s) base64 to bytes failed:%v", fieldName, err)
	}
	return bytes
}

func (c testCase) addressFiled(t *testing.T, fieldName string) address.Address {
	addStr := c.stringField(t, fieldName)
	addr, err := address.NewFromString(addStr)
	if err != nil {
		t.Fatalf("decode string(%s) to address failed:%v", addStr, err)
	}
	return addr
}

func (c testCase) numField(t *testing.T, fieldName string) int64 {
	num, isok := c[fieldName]
	if !isok {
		t.Fatalf("can't find field:%s", fieldName)
	}

	n, isok := num.(float64)
	if !isok {
		t.Fatalf("field(%s) is not number", fieldName)
	}
	return int64(n)
}

func (c testCase) msgid() string {
	if msgCid, isok := c["cid"]; isok {
		if s, isok := msgCid.(string); isok {
			return s
		}
	}
	return "not exist"
}

func (c testCase) receipt(t *testing.T) *types.MessageReceipt {
	// doesn't care about other filed
	return &types.MessageReceipt{
		Return: c.base64Field(t, "return"),
	}
}

func (c testCase) message(t *testing.T) *types.Message {
	return &types.Message{
		Version: 0,
		To:      c.addressFiled(t, "to"),
		Method:  abi.MethodNum(c.numField(t, "method")),
		Params:  c.base64Field(t, "params"),
	}
}

func (c testCase) wantArgs(t *testing.T, p reflect.Type) interface{} {
	if p == nil {
		return nil
	}
	if p.Kind() == reflect.Ptr {
		p = p.Elem()
	}
	i := reflect.New(p).Interface()
	data := c.base64Field(t, "params")
	if len(data) == 0 {
		data = []byte("{}")
	}
	if err := cbor.ReadCborRPC(bytes.NewReader(data), i); err != nil {
		require.NoError(t, err)
	}
	return i
}

func (c testCase) wantRet(t *testing.T, p reflect.Type) interface{} {
	if p == nil {
		return nil
	}
	if p.Kind() == reflect.Ptr {
		p = p.Elem()
	}
	i := reflect.New(p).Interface()
	data := c.base64Field(t, "return")
	if len(data) == 0 {
		data = []byte("{}")
	}
	if err := cbor.ReadCborRPC(bytes.NewReader(data), i); err != nil {
		require.NoError(t, err)
	}
	return i
}

func (c testCase) wantErr() bool {
	isErr, isok := c["is_err"]
	if isok {
		if b, isok := isErr.(bool); isok {
			return b
		}
	}
	return false
}

func TestMessagePaser_ParseMessage(t *testing.T) {
	tf.UnitTest(t)
	var tests []testCase
	file := "./test_cases_parsing_message.json"
	data, err := ioutil.ReadFile(file)
	require.NoErrorf(t, err, "read file:%s failed:%v", file, err)
	require.NoErrorf(t, json.Unmarshal(data, &tests), "unmarshal data to test cases failed")

	ms, err := NewMessageParser(newMockActorGetter(t))
	require.NoError(t, err)

	ctx := context.TODO()

	for _, tt := range tests {
		t.Run(tt.name(t), func(t *testing.T) {
			msgID := tt.msgid()
			msg := tt.message(t)
			gotArgs, gotRets, err := ms.ParseMessage(ctx, msg, tt.receipt(t))
			if (err != nil) != tt.wantErr() {
				t.Errorf("ParseMessage(%s) error = %v, wantErr %v\n%#v",
					tt.msgid(), err, tt.wantErr(), msg)
				return
			}

			if tt.wantErr() {
				return
			}
			wantArgs := tt.wantArgs(t, reflect.TypeOf(gotArgs))
			if !reflect.DeepEqual(gotArgs, wantArgs) {
				t.Errorf("ParseMessage(%v) gotArgs = %v, want %v", msgID, gotArgs, wantArgs)
			}

			wantRets := tt.wantRet(t, reflect.TypeOf(gotRets))
			if !reflect.DeepEqual(gotRets, wantRets) {
				t.Errorf("ParseMessage(%s) gotRet = %v, want %v", msgID, gotRets, wantRets)
			}
		})
	}
}
