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

func (app *BaseApp) PublishBlockEvents(flush PublishEventFlush) {
	if app.EnablePublish {
		app.PublishEvents <- flush
	}
}
