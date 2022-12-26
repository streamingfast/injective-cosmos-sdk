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

const ABCI_PREFIX = "abci_"

type AppLifeCycle = uint8

const (
	AppBeginBlocker = 0
	AppTx           = 1
	AppEndBlocker   = 2
)

func NewEventStore(dbPath string) *EventStore {
	if dbPath == "" {
		return nil
	}

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

func (es *EventStore) SetBlockEvents(phase AppLifeCycle, height int64, abciEvents sdk.Events, protoEvents []codectypes.Any) {
	// reset index
	if phase == AppBeginBlocker {
		es.eventIndex = 0
		es.height = height
	}

	h := []byte(string(es.height))
	prefixStore := prefix.NewStore(es.store, h)

	// write abci events
	for _, event := range abciEvents {
		anyEvent := parseAbciEvent(ABCI_PREFIX+event.Type, event)
		e, _ := anyEvent.Marshal()
		prefixStore.Set([]byte(string(es.eventIndex)), e)
		es.eventIndex += 1
	}

	// write typed events
	for _, event := range protoEvents {
		e, _ := event.Marshal()
		prefixStore.Set([]byte(string(es.eventIndex)), e)
		es.eventIndex += 1
	}
}

func (es *EventStore) Commit() {
	es.cms.Commit()
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
