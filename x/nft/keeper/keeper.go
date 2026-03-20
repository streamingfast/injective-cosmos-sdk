package keeper

import (
	"context"

	"cosmossdk.io/core/address"
	store "cosmossdk.io/core/store"
	"cosmossdk.io/x/nft"
	metrics "github.com/InjectiveLabs/metrics/v2"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Keeper of the nft store
type Keeper struct {
	meter        metrics.Meter
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	bk           nft.BankKeeper
	ac           address.Codec
}

// NewKeeper creates a new nft Keeper instance
func NewKeeper(storeService store.KVStoreService,
	cdc codec.BinaryCodec, ak nft.AccountKeeper, bk nft.BankKeeper,
) Keeper {
	// ensure nft module account is set
	if addr := ak.GetModuleAddress(nft.ModuleName); addr == nil {
		panic("the nft module account has not been set")
	}

	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		bk:           bk,
		ac:           ak.AddressCodec(),
	}
}

func (k *Keeper) Meter(ctx context.Context) metrics.Meter {
	if k.meter == nil {
		k.meter = sdk.UnwrapSDKContext(ctx).Meter().SubMeter(nft.ModuleName, metrics.Tag("svc", nft.ModuleName))
	}

	return k.meter
}
