package baseapp_test

import (
	"fmt"
	"testing"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/api/cometbft/types/v1"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/baseapp"
	baseapptestutil "github.com/cosmos/cosmos-sdk/baseapp/testutil"
	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ proto.Message = StringPublishEvent{}

type StringPublishEvent struct {
	data string
}

func (e StringPublishEvent) Reset() {
	e = StringPublishEvent{}
}

func (e StringPublishEvent) String() string {
	return e.data
}

func (StringPublishEvent) ProtoMessage() {}

func getStreamEventFlushChan(app *baseapp.BaseApp) chan baseapp.PublishEventFlush {
	app.EnablePublish = true
	publishEventChan := make(chan baseapp.PublishEventFlush)

	go func() {
		for {
			select {
			case e := <-app.PublishEvents:
				publishEventChan <- e
			}
		}
	}()

	return publishEventChan
}

func TestPublishEvent_FinalizeBlock_WithBeginAndEndBlocker(t *testing.T) {
	name := t.Name()
	db := dbm.NewMemDB()
	app := baseapp.NewBaseApp(name, log.NewTestLogger(t), db, nil)

	publishEventChan := getStreamEventFlushChan(app)

	app.SetBeginBlocker(func(ctx sdk.Context) (sdk.BeginBlock, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())
		ctx.EventManager().EmitEvent(types.Event{
			Type: "sometype",
			Attributes: []abci.EventAttribute{
				{
					Key:   "foo",
					Value: "bar",
				},
			},
		})
		ctx.PublishEventManager().EmitEvent(StringPublishEvent{"sometype2"})

		return sdk.BeginBlock{
			Events: ctx.EventManager().ABCIEvents(),
		}, nil
	})

	app.SetEndBlocker(func(ctx sdk.Context) (sdk.EndBlock, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())
		ctx.EventManager().EmitEvent(types.Event{
			Type: "anothertype",
			Attributes: []abci.EventAttribute{
				{
					Key:   "foo",
					Value: "bar",
				},
			},
		})

		ctx.PublishEventManager().EmitEvent(StringPublishEvent{"anothertype2"})

		return sdk.EndBlock{
			Events: ctx.EventManager().ABCIEvents(),
		}, nil
	})

	_, err := app.InitChain(
		&abci.InitChainRequest{
			InitialHeight: 1,
		},
	)
	require.NoError(t, err)

	res, err := app.FinalizeBlock(&abci.FinalizeBlockRequest{Height: 1})
	require.NoError(t, err)

	require.Len(t, res.Events, 2)

	require.Equal(t, "sometype", res.Events[0].Type)
	require.Equal(t, "foo", res.Events[0].Attributes[0].Key)
	require.Equal(t, "bar", res.Events[0].Attributes[0].Value)
	require.Equal(t, "mode", res.Events[0].Attributes[1].Key)
	require.Equal(t, "BeginBlock", res.Events[0].Attributes[1].Value)

	require.Equal(t, "anothertype", res.Events[1].Type)
	require.Equal(t, "foo", res.Events[1].Attributes[0].Key)
	require.Equal(t, "bar", res.Events[1].Attributes[0].Value)
	require.Equal(t, "mode", res.Events[1].Attributes[1].Key)
	require.Equal(t, "EndBlock", res.Events[1].Attributes[1].Value)

	_, err = app.Commit()
	require.NoError(t, err)

	require.Equal(t, int64(1), app.LastBlockHeight())

	pevts := <-publishEventChan
	require.Len(t, pevts.BlockEvents.PublishEvents, 2)
	require.Equal(t, StringPublishEvent{"sometype2"}, pevts.BlockEvents.PublishEvents[0])
	require.Equal(t, StringPublishEvent{"anothertype2"}, pevts.BlockEvents.PublishEvents[1])

	require.Len(t, pevts.BlockEvents.AbciEvents, 2)

	require.Len(t, pevts.BlockEvents.TrueOrder, 4)
	require.Equal(t, pevts.BlockEvents.TrueOrder, []baseapp.EventType{
		baseapp.EventTypeAbci, baseapp.EventTypePublish, baseapp.EventTypeAbci, baseapp.EventTypePublish,
	})

}

func TestPublishEvent_FinalizeBlock_DeliverTx(t *testing.T) {
	anteKey := []byte("ante-key")
	anteOpt := func(bapp *baseapp.BaseApp) {
		bapp.SetAnteHandler(anteHandlerTxTestWithCustomEventEmit(t, capKey1, anteKey, true))
	}
	suite := NewBaseAppSuite(t, anteOpt)
	publishEventChan := getStreamEventFlushChan(suite.baseApp)

	_, err := suite.baseApp.InitChain(&abci.InitChainRequest{
		ConsensusParams: &cmtproto.ConsensusParams{},
	})
	require.NoError(t, err)

	deliverKey := []byte("deliver-key")
	baseapptestutil.RegisterCounterServer(suite.baseApp.MsgServiceRouter(), CounterServerImpl{t, capKey1, deliverKey, true})

	nBlocks := 3
	txPerHeight := 5

	var lastAppHash []byte

	for blockN := 0; blockN < nBlocks; blockN++ {

		txs := [][]byte{}
		for i := 0; i < txPerHeight; i++ {
			counter := int64(blockN*txPerHeight + i)
			tx := newTxCounter(t, suite.txConfig, counter, counter)

			txBytes, err := suite.txConfig.TxEncoder()(tx)
			require.NoError(t, err)

			txs = append(txs, txBytes)
		}

		res, err := suite.baseApp.FinalizeBlock(&abci.FinalizeBlockRequest{
			Height: int64(blockN) + 1,
			Txs:    txs,
		})
		require.NoError(t, err)

		for i := 0; i < txPerHeight; i++ {
			counter := int64(blockN*txPerHeight + i)
			require.True(t, res.TxResults[i].IsOK(), fmt.Sprintf("%v", res))

			events := res.TxResults[i].GetEvents()
			require.Len(t, events, 3, "should contain ante handler, message type and counter events respectively")
			require.Equal(t, sdk.MarkEventsToIndex(counterEvent("ante_handler", counter).ToABCIEvents(), map[string]struct{}{})[0], events[0], "ante handler event")
			require.Equal(t, sdk.MarkEventsToIndex(counterEvent(sdk.EventTypeMessage, counter).ToABCIEvents(), map[string]struct{}{})[0].Attributes[0], events[2].Attributes[0], "msg handler update counter event")
		}

		_, err = suite.baseApp.Commit()
		require.NoError(t, err)

		pevts := <-publishEventChan
		require.Equal(t, pevts.Height, int64(blockN+1))
		if blockN > 0 {
			require.Equal(t, pevts.PrevAppHash, lastAppHash, "should be the same as last app hash")
		}
		require.Equal(t, pevts.NewAppHash, res.AppHash)
		require.Len(t, pevts.TxEvents, txPerHeight)
		fmt.Println(pevts.TxEvents)
		for i := 0; i < txPerHeight; i++ {
			require.Len(t, pevts.TxEvents[i].PublishEvents, 2)
			require.Len(t, pevts.TxEvents[i].TrueOrder, 5)
		}

		lastAppHash = res.AppHash
	}
}

func TestPublishEvent_FinalizeBlock_FailedMessageDiscardsPublishEvents(t *testing.T) {
	anteKey := []byte("ante-key")
	deliverKey := []byte("deliver-key")
	suite := NewBaseAppSuite(t, func(app *baseapp.BaseApp) {
		app.SetAnteHandler(anteHandlerTxTestWithCustomEventEmit(t, capKey1, anteKey, true))
	})
	app := suite.baseApp
	app.EnablePublish = true
	app.PublishEvents = make(chan baseapp.PublishEventFlush, 1)

	_, err := app.InitChain(&abci.InitChainRequest{
		ConsensusParams: &cmtproto.ConsensusParams{},
	})
	require.NoError(t, err)
	baseapptestutil.RegisterCounterServer(app.MsgServiceRouter(), CounterServerImpl{t, capKey1, deliverKey, true})

	tx := newTxCounter(t, suite.txConfig, 0, 0, 1)
	msgs := tx.GetMsgs()
	msgs[1].(*baseapptestutil.MsgCounter).FailOnHandler = true
	builder := suite.txConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msgs...))
	builder.SetMemo(tx.GetMemo())
	setTxSignature(t, builder, 0)
	txBytes, err := suite.txConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)

	res, err := app.FinalizeBlock(&abci.FinalizeBlockRequest{Height: 1, Txs: [][]byte{txBytes}})
	require.NoError(t, err)
	require.Len(t, res.TxResults, 1)
	require.False(t, res.TxResults[0].IsOK())

	store := app.GetContextForFinalizeBlock(nil).KVStore(capKey1)
	require.Equal(t, int64(1), getIntFromStore(t, store, anteKey), "ante state remains committed")
	require.Equal(t, int64(0), getIntFromStore(t, store, deliverKey), "message state is rolled back")

	_, err = app.Commit()
	require.NoError(t, err)
	flush := <-app.PublishEvents
	require.Len(t, flush.TxEvents, 1)
	require.Equal(t, []proto.Message{
		NewTestPublishEvent("ante_handler", map[string]string{"counter": "0"}),
	}, flush.TxEvents[0].PublishEvents, "only committed ante events are published")
	require.Equal(t, []baseapp.EventType{
		baseapp.EventTypeAbci, baseapp.EventTypePublish,
	}, flush.TxEvents[0].TrueOrder)
}
