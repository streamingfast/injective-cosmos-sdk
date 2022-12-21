package eventstore

import (
	"encoding/json"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/store"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	dbm "github.com/tendermint/tm-db"
)

type EventStore struct {
	cms        storetypes.CommitMultiStore
	store      storetypes.CommitKVStore
	eventIndex uint
	height     int64
}

var validAbciEventMap = map[string]string{
	"transfer": "transfer",
}

func NewEventStore(dbPath string) *EventStore {
	db, err := dbm.NewGoLevelDB(KeyEventProviderDB, dbPath)
	if err != nil {
		panic(err)
	}
	cms := store.NewCommitMultiStore(db)
	storeKey := sdk.NewKVStoreKey(KeyEventProviderStore)
	cms.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	cms.LoadLatestVersion()
	if err = cms.LoadLatestVersion(); err != nil {
		panic(err)
	}

	store := cms.GetCommitKVStore(storeKey)

	return &EventStore{
		cms:        cms,
		store:      store,
		eventIndex: 0,
	}
}

func (ep *EventStore) GetBlockEvents(height int64) [][]byte {
	h := []byte(string(height))
	prefixStore := prefix.NewStore(ep.store, h)

	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	events := [][]byte{}
	for ; iterator.Valid(); iterator.Next() {
		events = append(events, iterator.Value())
	}

	return events
}

func (ep *EventStore) SetBeginBlockerEvents(height int64, abciEvents sdk.Events, protoEvents []codectypes.Any) {
	h := []byte(string(height))
	prefixStore := prefix.NewStore(ep.store, h)

	ep.eventIndex = 0
	ep.height = height

	for _, event := range abciEvents {
		if name, ok := validAbciEventMap[event.Type]; ok {
			anyEvent := parseAbciEvent(name, event)
			e, _ := anyEvent.Marshal()
			prefixStore.Set([]byte(string(ep.eventIndex)), e)
			ep.eventIndex += 1
		}
	}

	for _, event := range protoEvents {
		e, _ := event.Marshal()
		prefixStore.Set([]byte(string(ep.eventIndex)), e)
		ep.eventIndex += 1
	}
}

func (ep *EventStore) SetTxEvents(abciEvents sdk.Events, protoEvents []codectypes.Any) {
	h := []byte(string(ep.height))
	prefixStore := prefix.NewStore(ep.store, h)

	for _, event := range abciEvents {
		if name, ok := validAbciEventMap[event.Type]; ok {
			anyEvent := parseAbciEvent(name, event)
			e, _ := anyEvent.Marshal()
			prefixStore.Set([]byte(string(ep.eventIndex)), e)
			ep.eventIndex += 1
		}
	}

	for _, event := range protoEvents {
		e, _ := event.Marshal()
		prefixStore.Set([]byte(string(ep.eventIndex)), e)
		ep.eventIndex += 1
	}
}

func (ep *EventStore) SetEndBlockerEvents(abciEvents sdk.Events, protoEvents []codectypes.Any) {
	h := []byte(string(ep.height))
	prefixStore := prefix.NewStore(ep.store, h)

	for _, event := range abciEvents {
		if name, ok := validAbciEventMap[event.Type]; ok {
			anyEvent := parseAbciEvent(name, event)
			e, _ := anyEvent.Marshal()
			prefixStore.Set([]byte(string(ep.eventIndex)), e)
			ep.eventIndex += 1
		}
	}

	for _, event := range protoEvents {
		e, _ := event.Marshal()
		prefixStore.Set([]byte(string(ep.eventIndex)), e)
		ep.eventIndex += 1
	}
}

func (ep *EventStore) Commit() {
	ep.cms.Commit()
}

func parseAbciEvent(name string, event sdk.Event) codectypes.Any {
	m := map[string]interface{}{}
	for _, a := range event.Attributes {
		m[string(a.Key)] = string(a.Value)
	}
	bz, _ := json.Marshal(m)
	e := codectypes.Any{
		TypeUrl: name,
		Value:   bz,
	}

	return e
}
