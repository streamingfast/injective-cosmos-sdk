package eventprovider

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/store"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	dbm "github.com/tendermint/tm-db"
)

type EventProviderStore struct {
	cms        storetypes.CommitMultiStore
	store      storetypes.CommitKVStore
	eventIndex uint
	height     int64
}

func NewEventProviderStore(dbPath string) *EventProviderStore {
	db, err := dbm.NewGoLevelDB(KeyEventProviderDB, "/Users/nam/Desktop/injective/injective-core")
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

	return &EventProviderStore{
		cms:        cms,
		store:      store,
		eventIndex: 0,
	}
}

func (ep *EventProviderStore) GetBlockEvents(height int64) [][]byte {
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

func (ep *EventProviderStore) SetBeginBlockerEvents(height int64, events []codectypes.Any) {
	h := []byte(string(height))
	prefixStore := prefix.NewStore(ep.store, h)

	ep.eventIndex = 0
	ep.height = height

	for _, event := range events {
		e, _ := event.Marshal()
		prefixStore.Set([]byte(string(ep.eventIndex)), e)
		ep.eventIndex += 1
	}
}

func (ep *EventProviderStore) SetTxEvents(events []codectypes.Any) {
	h := []byte(string(ep.height))
	prefixStore := prefix.NewStore(ep.store, h)

	for _, event := range events {
		e, _ := event.Marshal()
		prefixStore.Set([]byte(string(ep.eventIndex)), e)
		ep.eventIndex += 1
	}
}

func (ep *EventProviderStore) SetEndBlockerEvents(events []codectypes.Any) {
	h := []byte(string(ep.height))
	prefixStore := prefix.NewStore(ep.store, h)

	for _, event := range events {
		e, _ := event.Marshal()
		prefixStore.Set([]byte(string(ep.eventIndex)), e)
		ep.eventIndex += 1
	}
}

func (ep *EventProviderStore) Commit() {
	ep.cms.Commit()
}
