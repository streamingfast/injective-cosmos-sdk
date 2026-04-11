package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
)

// InitGenesis initializes the bank module's state from a given genesis state.
func (k BaseKeeper) InitGenesis(ctx context.Context, genState *types.GenesisState) {
	var err error
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "InitGenesis")(&err)

	if err = k.SetParams(sdkCtx, genState.Params); err != nil {
		panic(err)
	}

	for _, se := range genState.GetAllSendEnabled() {
		k.SetSendEnabled(sdkCtx, se.Denom, se.Enabled)
	}
	totalSupplyMap := sdk.NewMapCoins(sdk.Coins{})

	genState.Balances = types.SanitizeGenesisBalances(genState.Balances)

	for _, balance := range genState.Balances {
		addr := balance.GetAddress()
		bz, err := k.ak.AddressCodec().StringToBytes(addr)
		if err != nil {
			panic(err)
		}

		for _, coin := range balance.Coins {
			err = k.Balances.Set(sdkCtx, collections.Join(sdk.AccAddress(bz), coin.Denom), coin.Amount)
			if err != nil {
				panic(err)
			}
		}

		totalSupplyMap.Add(balance.Coins...)
	}
	totalSupply := totalSupplyMap.ToCoins()

	if !genState.Supply.Empty() && !genState.Supply.Equal(totalSupply) {
		err = fmt.Errorf("genesis supply is incorrect, expected %v, got %v", genState.Supply, totalSupply)
		panic(err)
	}

	for _, supply := range totalSupply {
		k.setSupply(sdkCtx, supply)
	}

	for _, meta := range genState.DenomMetadata {
		k.SetDenomMetaData(sdkCtx, meta)
	}
}

// ExportGenesis returns the bank module's genesis state.
func (k BaseKeeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	var err error
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "ExportGenesis")(&err)

	totalSupply, _, err := k.GetPaginatedTotalSupply(sdkCtx, &query.PageRequest{Limit: query.PaginationMaxLimit})
	if err != nil {
		err = fmt.Errorf("unable to fetch total supply %v", err)
		panic(err)
	}

	rv := types.NewGenesisState(
		k.GetParams(sdkCtx),
		k.GetAccountsBalances(sdkCtx),
		totalSupply,
		k.GetAllDenomMetaData(sdkCtx),
		k.GetAllSendEnabledEntries(sdkCtx),
	)
	return rv
}
