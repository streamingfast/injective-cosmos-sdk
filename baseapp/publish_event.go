package baseapp

import (
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/gogoproto/proto"
)

type EventType byte

const (
	EventTypeAbci EventType = iota
	EventTypePublish
)

type EventSet struct {
	AbciEvents    []abci.Event
	PublishEvents []proto.Message
	TrueOrder     []EventType
}

type PublishEventFlush struct {
	Height      int64
	PrevAppHash []byte
	NewAppHash  []byte
	BlockEvents EventSet
	TxEvents    []EventSet
}

func (app *BaseApp) publishEvents(commitHash []byte) {
	app.flushData.NewAppHash = commitHash

	if app.EnablePublish {
		app.PublishEvents <- app.flushData
	}
}
