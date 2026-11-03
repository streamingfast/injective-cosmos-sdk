package baseapp_test

import (
	"errors"
	"fmt"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/api/cometbft/types/v1"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/baseapp"
	baseapptestutil "github.com/cosmos/cosmos-sdk/baseapp/testutil"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

type removeErrorMempool struct {
	mempool.NoOpMempool
	err   error
	calls int
}

func (m *removeErrorMempool) Remove(sdk.Tx) error {
	m.calls++
	return m.err
}

func TestABCI_FinalizeBlock_MempoolRemoveError(t *testing.T) {
	for _, failOnHandler := range []bool{false, true} {
		t.Run(fmt.Sprintf("handlerFailure=%t", failOnHandler), func(t *testing.T) {
			pool := &removeErrorMempool{err: errors.New("local mempool removal failed")}
			suite := NewBaseAppSuite(t, baseapp.SetMempool(pool))
			key := []byte("message-counter")
			baseapptestutil.RegisterCounterServer(suite.baseApp.MsgServiceRouter(), CounterServerImpl{t, capKey1, key, false})
			_, err := suite.baseApp.InitChain(&abci.InitChainRequest{ConsensusParams: &cmtproto.ConsensusParams{}})
			require.NoError(t, err)
			_, _, addr := testdata.KeyTestPubAddr()
			builder := suite.txConfig.NewTxBuilder()
			require.NoError(t, builder.SetMsgs(&baseapptestutil.MsgCounter{Signer: addr.String(), FailOnHandler: failOnHandler}))
			setTxSignature(t, builder, 0)
			txBytes, err := suite.txConfig.TxEncoder()(builder.GetTx())
			require.NoError(t, err)

			res, err := suite.baseApp.FinalizeBlock(&abci.FinalizeBlockRequest{Height: 1, Txs: [][]byte{txBytes}})
			require.NoError(t, err)
			require.Len(t, res.TxResults, 1)
			wantCounter := int64(1)
			if failOnHandler {
				wantCounter = 0
				require.EqualValues(t, sdkerrors.ErrInvalidRequest.ABCICode(), res.TxResults[0].Code)
				require.Contains(t, res.TxResults[0].Log, "message handler failure")
			} else {
				require.Zero(t, res.TxResults[0].Code, res.TxResults[0].Log)
			}
			require.NotContains(t, res.TxResults[0].Log, pool.err.Error())
			require.Equal(t, 1, pool.calls)
			_, err = suite.baseApp.Commit()
			require.NoError(t, err)
			ctx, err := suite.baseApp.CreateQueryContext(1, false)
			require.NoError(t, err)
			require.Equal(t, wantCounter, getIntFromStore(t, ctx.KVStore(capKey1), key))
			require.Contains(t, suite.logBuffer.String(), pool.err.Error())
		})
	}
}
