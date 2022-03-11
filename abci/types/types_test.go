package types_test

import (
	fmt "fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
)

func TestHashAndProveResults(t *testing.T) {
	trs := []*abci.ExecTxResult{
		{Code: 0, Data: nil},
		{Code: 0, Data: []byte{}},
		{Code: 0, Data: []byte("one")},
		{Code: 14, Data: nil},
		{Code: 14, Data: []byte("foo")},
		{Code: 14, Data: []byte("bar")},
	}

	// Nil and []byte{} should produce the same bytes
	bz0, err := trs[0].Marshal()
	require.NoError(t, err)
	bz1, err := trs[1].Marshal()
	require.NoError(t, err)
	require.Equal(t, bz0, bz1)

	// Make sure that we can get a root hash from results and verify proofs.
	rs, err := abci.TxResultsToByteSlices(trs)
	require.NoError(t, err)
	root := merkle.HashFromByteSlices(rs)
	assert.NotEmpty(t, root)

	_, proofs := merkle.ProofsFromByteSlices(rs)
	for i, tr := range trs {
		bz, err := tr.Marshal()
		require.NoError(t, err)

		valid := proofs[i].Verify(root, bz)
		assert.NoError(t, valid, "%d", i)
	}
}

func TestHashDeterministicFieldsOnly(t *testing.T) {
	tr1 := abci.ExecTxResult{
		Code:      1,
		Data:      []byte("transaction"),
		Log:       "nondeterministic data: abc",
		Info:      "nondeterministic data: abc",
		GasWanted: 1000,
		GasUsed:   1000,
		Events:    []abci.Event{},
		Codespace: "nondeterministic.data.abc",
	}
	tr2 := abci.ExecTxResult{
		Code:      1,
		Data:      []byte("transaction"),
		Log:       "nondeterministic data: def",
		Info:      "nondeterministic data: def",
		GasWanted: 1000,
		GasUsed:   1000,
		Events:    []abci.Event{},
		Codespace: "nondeterministic.data.def",
	}
	r1, err := abci.TxResultsToByteSlices([]*abci.ExecTxResult{&tr1})
	require.NoError(t, err)
	r2, err := abci.TxResultsToByteSlices([]*abci.ExecTxResult{&tr2})
	require.NoError(t, err)
	require.Equal(t, merkle.HashFromByteSlices(r1), merkle.HashFromByteSlices(r2))
}

func TestValidateResponsePrepareProposal(t *testing.T) {
	t.Run("should error on total transaction size exceeding max data size", func(t *testing.T) {
		rpp := &abci.ResponsePrepareProposal{
			ModifiedTx: true,
			TxRecords: []*abci.TxRecord{
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{1, 2, 3, 4, 5},
				},
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{6, 7, 8, 9, 10},
				},
			},
		}
		err := rpp.Validate(9, [][]byte{})
		require.Error(t, err)
	})
	t.Run("should error on duplicate transactions with the same action", func(t *testing.T) {
		rpp := &abci.ResponsePrepareProposal{
			ModifiedTx: true,
			TxRecords: []*abci.TxRecord{
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{1, 2, 3, 4, 5},
				},
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{100},
				},
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{1, 2, 3, 4, 5},
				},
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{200},
				},
			},
		}
		err := rpp.Validate(100, [][]byte{})
		require.Error(t, err)
	})
	t.Run("should error on duplicate transactions with mixed actions", func(t *testing.T) {
		rpp := &abci.ResponsePrepareProposal{
			ModifiedTx: true,
			TxRecords: []*abci.TxRecord{
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{1, 2, 3, 4, 5},
				},
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{100},
				},
				{
					Action: abci.TxRecord_REMOVED,
					Tx:     []byte{1, 2, 3, 4, 5},
				},
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{200},
				},
			},
		}
		err := rpp.Validate(100, [][]byte{})
		require.Error(t, err)
	})
	t.Run("should error on new transactions marked UNMODIFIED", func(t *testing.T) {
		rpp := &abci.ResponsePrepareProposal{
			ModifiedTx: true,
			TxRecords: []*abci.TxRecord{
				{
					Action: abci.TxRecord_UNMODIFIED,
					Tx:     []byte{1, 2, 3, 4, 5},
				},
			},
		}
		err := rpp.Validate(100, [][]byte{})
		fmt.Println(err)
		require.Error(t, err)
	})
	t.Run("should error on new transactions marked REMOVED", func(t *testing.T) {
		rpp := &abci.ResponsePrepareProposal{
			ModifiedTx: true,
			TxRecords: []*abci.TxRecord{
				{
					Action: abci.TxRecord_REMOVED,
					Tx:     []byte{1, 2, 3, 4, 5},
				},
			},
		}
		err := rpp.Validate(100, [][]byte{})
		fmt.Println(err)
		require.Error(t, err)
	})
	t.Run("should error on existing transaction marked as ADDED", func(t *testing.T) {
		rpp := &abci.ResponsePrepareProposal{
			ModifiedTx: true,
			TxRecords: []*abci.TxRecord{
				{
					Action: abci.TxRecord_ADDED,
					Tx:     []byte{1, 2, 3, 4, 5},
				},
			},
		}
		err := rpp.Validate(100, [][]byte{{1, 2, 3, 4, 5}})
		require.Error(t, err)
	})
	t.Run("should error if any transaction marked as UNKNOWN", func(t *testing.T) {
		rpp := &abci.ResponsePrepareProposal{
			ModifiedTx: true,
			TxRecords: []*abci.TxRecord{
				{
					Action: abci.TxRecord_UNKNOWN,
					Tx:     []byte{1, 2, 3, 4, 5},
				},
			},
		}
		err := rpp.Validate(100, [][]byte{})
		require.Error(t, err)
	})
}