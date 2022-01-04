package chain

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestBlockHeaderMarshal(t *testing.T) {
	tf.UnitTest(t)
	const mdata = "f021344"
	const cdata = "bafy2bzaced35aqx5wnwp4ohegreumsheigitrhcqlr3lmz4phyzwikmi44sww"
	const bdata = "904400e0a601815860b48541b503b47535334553bf0d1fc702d395133e28b48bbae7550e955025ee09f642cb82c79de036099c657b7efb78e10b685aa99d817f6e560805787c5df894ac6758b0d68c72fb498113a5e763ef65d5bff576cd7fc3a847684a410b89c63c82025860a233b638f28312f9014728cf42e2c353bbf2031506488fa3192d137881e8b9666a2d131218e96b51b701f826940ef6630ae13b20c7bf115155e349c88363949fd7704f8026c3c3b1f872d9085e912856d2b56b8ce8a6fb8cf4e996aaa8476e2c81821a001506545860a699f4d93c54d66d46a46cab8da059fbf7ef82e40dd19a67fc1c5a402422304e7b8eec5c2dc6107e0cf1be676f12ce9b05675160e1c66f0ae5ebcf056e303ca39aa813acd5403844604b51f1e3dd5fcd271978346b85cdfa6d75cb46e2c6609581820358c09095232420f9caff389c0709ace12897ad0b5734b104011849e0f008febf39d8c64079236d3e75c24c08a613bcd946538a4b966b3c5a79cf61832c673f2ec90d22d02c16e28073c20995f5259567736d6e6f2fee588c7c23ca1946d753a783fe14404c3f4684a0a7cf5ceadc8a7a2cd9ad0387b96608eca6d3604ed7beb948f7fa2e235f1d611114f66752c6c36ac9aeb17c2f36d70accbd7554678034381486a1a95ea36db4dc549ee152a00c1b454da4f47b33327609be8b055f14681a2edf84d82a5827000171a0e4022084da38b952ab5644c5418c3305b3c22b5eca92eab9e23cdb675163773a964c53d82a5827000171a0e402201ef87dc542d008d961a30a36935a06d28ef05e4ed5e22f7779c4f3f8002c451dd82a5827000171a0e4022079bfefc62c740cda4b0463ceba68e9613c5c47ef7bbed968b179763ae7978bf4d82a5827000171a0e402202f9becc403d7228035f153d9cba42db472ed3c1cb3f22c7ae02eaaabca53e80f460001d7dbb8171a00067680d82a5827000171a0e402204f2120d6581f3d69a5d62e25dd993d1825ce6a446ffa801a4092e7e3a28d4b73d82a5827000171a0e402204bc482ae9a6a1afd1a252264a4bcae9fb2150faf9910b80703e9fbb91ab041e3d82a5827000171a0e4022017d2c80f5b157b61e96ea4ef3888762fa81a9a853bffa624f4bfb9c388859a88586102b3f7f6dc71591af0a61bbcad978178fc123a6edbec959716c028ec976b997df83af557a5ad1d05544d5ce82e5461c562196ea998b437bf0ceb7965871bd6d9e16a2df9cfaaf50b627f5a406d344f1ae0d8e0eaa5835f9c092fe24681cbc7761d1a618f1680586102920f0a831f86073b12641e6c880ddc2823a9c7b1b14b56f7995eaafc35df9c8f3066cd3ab9693c53b388e4c46d7680b50dcd242471d763a5114274c475eeb7d6561e35f51db3b6ac46c4fb8f4218ddc6d6fae3c1cd09fa70c21e6e87bd94e33100420064"
	const sdata = "904400e0a601815860b48541b503b47535334553bf0d1fc702d395133e28b48bbae7550e955025ee09f642cb82c79de036099c657b7efb78e10b685aa99d817f6e560805787c5df894ac6758b0d68c72fb498113a5e763ef65d5bff576cd7fc3a847684a410b89c63c82025860a233b638f28312f9014728cf42e2c353bbf2031506488fa3192d137881e8b9666a2d131218e96b51b701f826940ef6630ae13b20c7bf115155e349c88363949fd7704f8026c3c3b1f872d9085e912856d2b56b8ce8a6fb8cf4e996aaa8476e2c81821a001506545860a699f4d93c54d66d46a46cab8da059fbf7ef82e40dd19a67fc1c5a402422304e7b8eec5c2dc6107e0cf1be676f12ce9b05675160e1c66f0ae5ebcf056e303ca39aa813acd5403844604b51f1e3dd5fcd271978346b85cdfa6d75cb46e2c6609581820358c09095232420f9caff389c0709ace12897ad0b5734b104011849e0f008febf39d8c64079236d3e75c24c08a613bcd946538a4b966b3c5a79cf61832c673f2ec90d22d02c16e28073c20995f5259567736d6e6f2fee588c7c23ca1946d753a783fe14404c3f4684a0a7cf5ceadc8a7a2cd9ad0387b96608eca6d3604ed7beb948f7fa2e235f1d611114f66752c6c36ac9aeb17c2f36d70accbd7554678034381486a1a95ea36db4dc549ee152a00c1b454da4f47b33327609be8b055f14681a2edf84d82a5827000171a0e4022084da38b952ab5644c5418c3305b3c22b5eca92eab9e23cdb675163773a964c53d82a5827000171a0e402201ef87dc542d008d961a30a36935a06d28ef05e4ed5e22f7779c4f3f8002c451dd82a5827000171a0e4022079bfefc62c740cda4b0463ceba68e9613c5c47ef7bbed968b179763ae7978bf4d82a5827000171a0e402202f9becc403d7228035f153d9cba42db472ed3c1cb3f22c7ae02eaaabca53e80f460001d7dbb8171a00067680d82a5827000171a0e402204f2120d6581f3d69a5d62e25dd993d1825ce6a446ffa801a4092e7e3a28d4b73d82a5827000171a0e402204bc482ae9a6a1afd1a252264a4bcae9fb2150faf9910b80703e9fbb91ab041e3d82a5827000171a0e4022017d2c80f5b157b61e96ea4ef3888762fa81a9a853bffa624f4bfb9c388859a88586102b3f7f6dc71591af0a61bbcad978178fc123a6edbec959716c028ec976b997df83af557a5ad1d05544d5ce82e5461c562196ea998b437bf0ceb7965871bd6d9e16a2df9cfaaf50b627f5a406d344f1ae0d8e0eaa5835f9c092fe24681cbc7761d1a618f1680f600420064"

	maddr, err := address.NewFromString(mdata)
	require.NoErrorf(t, err, "parse miner address %s", mdata)

	c, err := cid.Decode(cdata)
	require.NoErrorf(t, err, "decode cid %s", cdata)

	require.NotEqual(t, bdata, sdata, "check raw sign bytes")

	b, err := hex.DecodeString(bdata)
	require.NoError(t, err, "decode block header binary")

	signb, err := hex.DecodeString(sdata)
	require.NoError(t, err, "decode sign bytes")

	bh, err := DecodeBlock(b)
	require.NoError(t, err, "decode block header")

	require.Equal(t, maddr, bh.Miner, "check for miner")
	signdata, err := bh.SignatureData()
	require.NoError(t, err, "call bh.SignatureData")
	require.Equal(t, signb, signdata, "check for signature data")

	require.Equal(t, c, bh.Cid(), "check for bh.Cid()")
	serialized, err := bh.Serialize()
	require.NoError(t, err, "call bh.Serialize")
	require.Equal(t, b, serialized, "check for bh.Serialize()")

	blk, err := bh.ToStorageBlock()
	require.NoError(t, err, "call bh.ToStorageBlock")

	require.Equal(t, c, blk.Cid(), "check for blk.Cid()")
	require.Equal(t, b, blk.RawData(), "check for blk.RawData()")
}

func TestBlockHeaderBasic(t *testing.T) {
	tf.UnitTest(t)
	var buf bytes.Buffer
	sliceLen := 5
	bytesLen := 32
	for i := 0; i < 64; i++ {
		var src, dst BlockHeader

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst)
			},

			ProvideOpts: []interface{}{
				testutil.WithSliceLen(sliceLen),
				testutil.BytesFixedProvider(bytesLen),
				testutil.IDAddressProvider(),
			},

			Provided: func() {
				require.Equal(t, src.Miner.Protocol(), address.ID, "miner addr proto")
				require.Len(t, src.Parents, sliceLen, "parents length")
				require.NotNil(t, src.ElectionProof, "ElectionProof")
				require.Len(t, src.ElectionProof.VRFProof, bytesLen, "VRFProof len")
				require.NotNil(t, src.BlockSig, "BlockSig")
				require.Len(t, src.BlockSig.Data, bytesLen, "BlockSig.Data len")
				require.NotNil(t, src.BLSAggregate, "BLSAggregate")
				require.Len(t, src.BLSAggregate.Data, bytesLen, "BLSAggregate.Data len")
			},

			Marshaled: func(b []byte) {
				decoded, err := DecodeBlock(b)
				require.NoError(t, err, "DecodeBlock")
				require.Equal(t, src, *decoded)
			},

			Finished: func() {
				require.Equal(t, src.LastTicket(), dst.LastTicket())
				require.Equal(t, src, dst)
				require.Equal(t, src.String(), dst.String())
				require.True(t, src.Equals(&dst))

				require.False(t, src.IsValidated(), "check validated before set")

				src.SetValidated()
				require.True(t, src.IsValidated(), "check validated before set")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
