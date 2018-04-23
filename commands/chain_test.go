package commands

import (
	"context"
	"encoding/json"
	"testing"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestChainHead(t *testing.T) {
	t.Run("returns an error if no best block", func(t *testing.T) {
		assert := assert.New(t)

		getter := func() *types.Block {
			return nil
		}

		emitter := NewMockEmitter(func(v interface{}) error {
			return nil
		})

		err := runChainHead(getter, emitter.emit)
		assert.Error(err)
		assert.Equal(0, len(emitter.calls()))
	})

	t.Run("emits the blockchain head", func(t *testing.T) {
		assert := assert.New(t)

		block := types.NewBlockForTest(nil, 1)

		getter := func() *types.Block {
			return block
		}

		emitter := NewMockEmitter(func(v interface{}) error {
			types.AssertCidsEqual(assert, block.Cid(), v.(cmds.Single).Value.(*cid.Cid))
			return nil
		})

		err := runChainHead(getter, emitter.emit)
		assert.NoError(err)
		assert.Equal(1, len(emitter.calls()))
	})
}

func TestChainLsRun(t *testing.T) {
	ctx := context.Background()

	t.Run("chain of height two", func(t *testing.T) {
		assert := assert.New(t)

		genBlock := types.NewBlockForTest(nil, 1)
		chlBlock := types.NewBlockForTest(genBlock, 2)

		getter := func(ctx context.Context) <-chan interface{} {
			out := make(chan interface{})

			go func() {
				defer close(out)

				bs := []*types.Block{chlBlock, genBlock}
				for _, b := range bs {
					out <- b
				}
			}()

			return out
		}

		emitter := NewMockEmitter(func(v interface{}) error {
			return nil
		})

		err := runChainLs(ctx, getter, emitter.emit)
		assert.NoError(err)

		assert.Equal(2, len(emitter.calls()))
		types.AssertHaveSameCid(assert, chlBlock, emitter.calls()[0].(*types.Block))
		types.AssertHaveSameCid(assert, genBlock, emitter.calls()[1].(*types.Block))
	})

	t.Run("emit best block and then fail getting parent", func(t *testing.T) {
		assert := assert.New(t)

		genBlock := types.NewBlockForTest(nil, 1)
		chlBlock := types.NewBlockForTest(genBlock, 2)

		getter := func(ctx context.Context) <-chan interface{} {
			out := make(chan interface{})

			go func() {
				defer close(out)

				out <- chlBlock
				out <- errors.New("context deadline exceeded")
			}()

			return out
		}

		emitter := NewMockEmitter(func(v interface{}) error {
			types.AssertHaveSameCid(assert, chlBlock, v.(*types.Block))
			return nil
		})

		err := runChainLs(ctx, getter, emitter.emit)
		assert.Error(err)
		assert.Contains(err.Error(), "context deadline exceeded")
		assert.Equal(1, len(emitter.calls()))
	})

	t.Run("JSON marshaling", func(t *testing.T) {
		assert := assert.New(t)

		parent := types.NewBlockForTest(nil, 0)
		child := types.NewBlockForTest(parent, 1)

		message := types.NewMessageForTestGetter()()
		receipt := types.NewMessageReceipt(types.SomeCid(), 123, "something terrible happened", []byte{1, 2, 3})
		child.Messages = []*types.Message{message}
		child.MessageReceipts = []*types.MessageReceipt{receipt}

		marshaled, e1 := json.Marshal(child)
		assert.NoError(e1)
		str := string(marshaled)

		assert.Contains(str, parent.Cid().String())
		assert.Contains(str, message.From.String())
		assert.Contains(str, message.To.String())

		// marshal/unmarshal symmetry
		var unmarshalled types.Block
		e2 := json.Unmarshal(marshaled, &unmarshalled)
		assert.NoError(e2)

		assert.Equal(uint8(123), unmarshalled.MessageReceipts[0].ExitCode)
		assert.Equal("something terrible happened", unmarshalled.MessageReceipts[0].Error)
		assert.Equal([]byte{1, 2, 3}, unmarshalled.MessageReceipts[0].Return)

		types.AssertHaveSameCid(assert, child, &unmarshalled)
	})
}
