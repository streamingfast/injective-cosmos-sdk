package baseapp

import (
	abci "github.com/cometbft/cometbft/abci/types"
	"time"
)

type StreamEvents struct {
	Events    []abci.Event
	Height    uint64
	BlockTime time.Time
	Flush     bool
	Source    string
}

func (app *BaseApp) AddStreamEvents(height int64, blockTime time.Time, events []abci.Event, flush bool, source string) {
	if app.EnableStreamer {
		app.StreamEvents <- StreamEvents{
			Events:    events,
			Height:    uint64(height),
			BlockTime: blockTime,
			Flush:     flush,
			Source:    source,
		}
	}
}
