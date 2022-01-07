package docgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"

	network2 "github.com/filecoin-project/go-state-types/network"
	v0 "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/venus-shared/api"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	textselector "github.com/ipld/go-ipld-selector-text-lite"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
)

var ExampleValues = map[reflect.Type]interface{}{
	reflect.TypeOf(auth.Permission("")): auth.Permission("write"),
	reflect.TypeOf(""):                  "string value",
	reflect.TypeOf(uint64(42)):          uint64(42),
	reflect.TypeOf(byte(7)):             byte(7),
	reflect.TypeOf([]byte{}):            []byte("byte array"),
}

func addExample(v interface{}) {
	ExampleValues[reflect.TypeOf(v)] = v
}

func init() {
	c, err := cid.Decode("bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4")
	if err != nil {
		panic(err)
	}

	ExampleValues[reflect.TypeOf(c)] = c

	c2, err := cid.Decode("bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve")
	if err != nil {
		panic(err)
	}

	tsk := types.NewTipSetKey(c, c2)

	ExampleValues[reflect.TypeOf(tsk)] = tsk

	addr, err := address.NewIDAddress(1234)
	if err != nil {
		panic(err)
	}

	ExampleValues[reflect.TypeOf(addr)] = addr

	pid, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	if err != nil {
		panic(err)
	}
	addExample(pid)
	addExample(&pid)
	addExample(network2.Version14)
	textSelExample := textselector.Expression("Links/21/Hash/Links/42/Hash")
	clientEvent := retrievalmarket.ClientEventDealAccepted
	addExample(bitfield.NewFromSet([]uint64{5}))
	addExample(abi.RegisteredSealProof_StackedDrg32GiBV1_1)
	addExample(abi.RegisteredPoStProof_StackedDrgWindow32GiBV1)
	addExample(abi.ChainEpoch(10101))
	addExample(crypto.SigTypeBLS)
	addExample(types.KTBLS)
	addExample(types.MTChainMsg)
	addExample(int64(9))
	addExample(12.3)
	addExample(123)
	addExample(uintptr(0))
	addExample(abi.MethodNum(1))
	addExample(exitcode.ExitCode(0))
	addExample(crypto.DomainSeparationTag_ElectionProofProduction)
	addExample(true)
	addExample(abi.UnpaddedPieceSize(1024))
	addExample(abi.UnpaddedPieceSize(1024).Padded())
	addExample(abi.DealID(5432))
	addExample(abi.SectorNumber(9))
	addExample(abi.SectorSize(32 * 1024 * 1024 * 1024))
	addExample(types.MpoolChange(0))
	addExample(network.Connected)
	addExample(types.NetworkName("mainnet"))
	addExample(types.SyncStateStage(1))
	addExample(api.FullAPIVersion1)
	addExample(types.PCHInbound)
	addExample(time.Minute)
	addExample(graphsync.RequestID(4))
	addExample(datatransfer.TransferID(3))
	addExample(datatransfer.Ongoing)
	addExample(clientEvent)
	addExample(&clientEvent)
	addExample(retrievalmarket.ClientEventDealAccepted)
	addExample(retrievalmarket.DealStatusNew)
	addExample(&textSelExample)
	addExample(network.ReachabilityPublic)
	addExample(map[string]int{"name": 42})
	addExample(map[string]time.Time{"name": time.Unix(1615243938, 0).UTC()})
	addExample(&types.ExecutionTrace{
		Msg:    ExampleValue("init", reflect.TypeOf(&types.Message{}), nil).(*types.Message),
		MsgRct: ExampleValue("init", reflect.TypeOf(&types.MessageReceipt{}), nil).(*types.MessageReceipt),
	})
	addExample(map[address.Address]*types.Actor{
		addr: {
			Code:    c,
			Head:    c2,
			Nonce:   10,
			Balance: abi.NewTokenAmount(100),
		},
	})
	addExample(map[string]types.MarketDeal{
		"t026363": ExampleValue("init", reflect.TypeOf(types.MarketDeal{}), nil).(types.MarketDeal),
	})
	addExample(map[string]types.MarketBalance{
		"t026363": ExampleValue("init", reflect.TypeOf(types.MarketBalance{}), nil).(types.MarketBalance),
	})
	addExample([]*types.EstimateMessage{
		{Msg: ExampleValue("init", reflect.TypeOf(&types.Message{}), nil).(*types.Message),
			Spec: ExampleValue("init", reflect.TypeOf(&types.MessageSendSpec{}), nil).(*types.MessageSendSpec),
		}})
	addExample(map[string]*pubsub.TopicScoreSnapshot{
		"/blocks": {
			TimeInMesh:               time.Minute,
			FirstMessageDeliveries:   122,
			MeshMessageDeliveries:    1234,
			InvalidMessageDeliveries: 3,
		},
	})
	addExample(map[string]metrics.Stats{
		"12D3KooWSXmXLJmBR1M7i9RW9GQPNUhZSzXKzxDHWtAgNuJAbyEJ": {
			RateIn:   100,
			RateOut:  50,
			TotalIn:  174000,
			TotalOut: 12500,
		},
	})
	addExample(map[protocol.ID]metrics.Stats{
		"/fil/hello/1.0.0": {
			RateIn:   100,
			RateOut:  50,
			TotalIn:  174000,
			TotalOut: 12500,
		},
	})
	maddr, err := multiaddr.NewMultiaddr("/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior")
	if err != nil {
		panic(err)
	}
	// because reflect.TypeOf(maddr) returns the concrete type...
	ExampleValues[reflect.TypeOf(struct{ A multiaddr.Multiaddr }{}).Field(0).Type] = maddr
	si := uint64(12)
	addExample(&si)
	addExample(retrievalmarket.DealID(5))
	addExample(abi.ActorID(1000))
	addExample(map[abi.SectorNumber]string{
		123: "can't acquire read lock",
	})
	addExample([]abi.SectorNumber{123, 124})
	// addExample(apitypes.OpenRPCDocument{
	// 	"openrpc": "1.2.6",
	// 	"info": map[string]interface{}{
	// 		"title":   "Lotus RPC API",
	// 		"version": "1.2.1/generated=2020-11-22T08:22:42-06:00",
	// 	},
	// 	"methods": []interface{}{}},
	// )
	addExample(types.CheckStatusCode(0))
	addExample(map[string]interface{}{"abc": 123})
}

func GetAPIType(name, pkg string) (i interface{}, t reflect.Type, permStruct []reflect.Type) {

	switch pkg {
	case "v1": // latest
		switch name {
		case "FullNode":
			i = &v1.FullNodeStruct{}
			t = reflect.TypeOf(new(struct{ v1.FullNode })).Elem()
			permStruct = append(permStruct, reflect.TypeOf(v1.IBlockStoreStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IBeaconStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IChainStruct{}.IAccountStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IChainStruct{}.IActorStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IChainStruct{}.IBeaconStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IChainStruct{}.IMinerStateStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IChainStruct{}.IChainInfoStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IMarketStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IMiningStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IMessagePoolStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IMultiSigStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.INetworkStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IPaychanStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.ISyncerStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IWalletStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v1.IJwtAuthAPIStruct{}.Internal))
		default:
			panic("unknown type")
		}
	case "v0":
		switch name {
		case "FullNode":
			i = &v0.FullNodeStruct{}
			t = reflect.TypeOf(new(struct{ v0.FullNode })).Elem()
			permStruct = append(permStruct, reflect.TypeOf(v0.IBlockStoreStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IBeaconStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IChainStruct{}.IAccountStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IChainStruct{}.IActorStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IChainStruct{}.IBeaconStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IChainStruct{}.IMinerStateStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IChainStruct{}.IChainInfoStruct.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IMarketStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IMiningStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IMessagePoolStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IMultiSigStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.INetworkStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IPaychanStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.ISyncerStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IWalletStruct{}.Internal))
			permStruct = append(permStruct, reflect.TypeOf(v0.IJwtAuthAPIStruct{}.Internal))
		default:
			panic("unknown type")
		}
	}
	return
}

func ExampleValue(method string, t, parent reflect.Type) interface{} {
	v, ok := ExampleValues[t]
	if ok {
		return v
	}

	switch t.Kind() {
	case reflect.Slice:
		out := reflect.New(t).Elem()
		out = reflect.Append(out, reflect.ValueOf(ExampleValue(method, t.Elem(), t)))
		return out.Interface()
	case reflect.Chan:
		return ExampleValue(method, t.Elem(), nil)
	case reflect.Struct:
		es := exampleStruct(method, t, parent)
		v := reflect.ValueOf(es).Elem().Interface()
		ExampleValues[t] = v
		return v
	case reflect.Array:
		out := reflect.New(t).Elem()
		for i := 0; i < t.Len(); i++ {
			out.Index(i).Set(reflect.ValueOf(ExampleValue(method, t.Elem(), t)))
		}
		return out.Interface()
	case reflect.Ptr:
		if t.Elem().Kind() == reflect.Struct {
			es := exampleStruct(method, t.Elem(), t)
			// ExampleValues[t] = es
			return es
		}
	case reflect.Interface:
		if t.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			return fmt.Errorf("empty error")
		}
		return struct{}{}
	}

	_, _ = fmt.Fprintf(os.Stderr, "Warnning: No example value for type: %s (method '%s')\n", t, method)
	return nil
}

func exampleStruct(method string, t, parent reflect.Type) interface{} {
	ns := reflect.New(t)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == parent {
			continue
		}
		if strings.Title(f.Name) == f.Name {
			ns.Elem().Field(i).Set(reflect.ValueOf(ExampleValue(method, f.Type, t)))
		}
	}

	return ns.Interface()
}

type Visitor struct {
	Root    string
	Methods map[string]ast.Node
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	st, ok := node.(*ast.TypeSpec)
	if !ok {
		return v
	}

	if st.Name.Name != v.Root {
		return nil
	}

	iface := st.Type.(*ast.InterfaceType)
	for _, m := range iface.Methods.List {
		if len(m.Names) > 0 {
			v.Methods[m.Names[0].Name] = m
		}
	}

	return v
}

const NoComment = "There are not yet any comments for this method."

func ParseApiASTInfo(apiFile, iface, pkg, dir string) (comments map[string]string, groupDocs map[string]string) {
	fset := token.NewFileSet()
	apiDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Println("./api filepath absolute error: ", err)
		return
	}
	apiFile, err = filepath.Abs(apiFile)
	if err != nil {
		fmt.Println("filepath absolute error: ", err, "file:", apiFile)
		return
	}
	pkgs, err := parser.ParseDir(fset, apiDir, nil, parser.AllErrors|parser.ParseComments)
	if err != nil {
		fmt.Println("parse error: ", err)
		return
	}

	ap := pkgs[pkg]

	f := ap.Files[apiFile]

	cmap := ast.NewCommentMap(fset, f, f.Comments)

	v := &Visitor{iface, make(map[string]ast.Node)}
	ast.Walk(v, ap)

	comments = make(map[string]string)
	groupDocs = make(map[string]string)
	for mn, node := range v.Methods {
		filteredComments := cmap.Filter(node).Comments()
		if len(filteredComments) == 0 {
			comments[mn] = NoComment
		} else {
			for _, c := range filteredComments {
				if strings.HasPrefix(c.Text(), "MethodGroup:") {
					parts := strings.Split(c.Text(), "\n")
					groupName := strings.TrimSpace(parts[0][12:])
					comment := strings.Join(parts[1:], "\n")
					groupDocs[groupName] = comment

					break
				}
			}

			l := len(filteredComments) - 1
			if len(filteredComments) > 1 {
				l = len(filteredComments) - 2
			}
			last := filteredComments[l].Text()
			if !strings.HasPrefix(last, "MethodGroup:") {
				comments[mn] = last
			} else {
				comments[mn] = NoComment
			}
		}
	}
	return comments, groupDocs
}

type MethodGroup struct {
	GroupName string
	Header    string
	Methods   []*Method
}

type Method struct {
	Comment         string
	Name            string
	InputExample    string
	ResponseExample string
}

func MethodGroupFromName(mn string) string {
	i := strings.IndexFunc(mn[1:], func(r rune) bool {
		return unicode.IsUpper(r)
	})
	if i < 0 {
		return ""
	}
	return mn[:i+1]
}
