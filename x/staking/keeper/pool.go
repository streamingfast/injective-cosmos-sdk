package keeper

import (
	"context"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GetBondedPool returns the bonded tokens pool's module account
func (k Keeper) GetBondedPool(ctx context.Context) (bondedPool sdk.ModuleAccountI) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetBondedPool")()

	return k.authKeeper.GetModuleAccount(sdkCtx, types.BondedPoolName)
}

// GetNotBondedPool returns the not bonded tokens pool's module account
func (k Keeper) GetNotBondedPool(ctx context.Context) (notBondedPool sdk.ModuleAccountI) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetNotBondedPool")()

	return k.authKeeper.GetModuleAccount(sdkCtx, types.NotBondedPoolName)
}

// bondedTokensToNotBonded transfers coins from the bonded to the not bonded pool within staking
func (k Keeper) bondedTokensToNotBonded(ctx context.Context, tokens math.Int) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "bondedTokensToNotBonded")()

	bondDenom, err := k.BondDenom(sdkCtx)
	if err != nil {
		return err
	}

	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, tokens))
	return k.bankKeeper.SendCoinsFromModuleToModule(sdkCtx, types.BondedPoolName, types.NotBondedPoolName, coins)
}

// notBondedTokensToBonded transfers coins from the not bonded to the bonded pool within staking
func (k Keeper) notBondedTokensToBonded(ctx context.Context, tokens math.Int) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "notBondedTokensToBonded")()

	bondDenom, err := k.BondDenom(sdkCtx)
	if err != nil {
		return err
	}

	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, tokens))
	return k.bankKeeper.SendCoinsFromModuleToModule(sdkCtx, types.NotBondedPoolName, types.BondedPoolName, coins)
}

// burnBondedTokens burns coins from the bonded pool module account
func (k Keeper) burnBondedTokens(ctx context.Context, amt math.Int) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "burnBondedTokens")()

	if !amt.IsPositive() {
		// skip as no coins need to be burned
		return nil
	}

	bondDenom, err := k.BondDenom(sdkCtx)
	if err != nil {
		return err
	}

	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, amt))

	return k.bankKeeper.BurnCoins(sdkCtx, types.BondedPoolName, coins)
}

// burnNotBondedTokens burns coins from the not bonded pool module account
func (k Keeper) burnNotBondedTokens(ctx context.Context, amt math.Int) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "burnNotBondedTokens")()

	if !amt.IsPositive() {
		// skip as no coins need to be burned
		return nil
	}

	bondDenom, err := k.BondDenom(sdkCtx)
	if err != nil {
		return err
	}

	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, amt))

	return k.bankKeeper.BurnCoins(sdkCtx, types.NotBondedPoolName, coins)
}

// TotalBondedTokens total staking tokens supply which is bonded
func (k Keeper) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "TotalBondedTokens")()

	bondedPool := k.GetBondedPool(sdkCtx)
	bondDenom, err := k.BondDenom(sdkCtx)
	if err != nil {
		return math.ZeroInt(), err
	}
	return k.bankKeeper.GetBalance(sdkCtx, bondedPool.GetAddress(), bondDenom).Amount, nil
}

// StakingTokenSupply staking tokens from the total supply
func (k Keeper) StakingTokenSupply(ctx context.Context) (math.Int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "StakingTokenSupply")()

	bondDenom, err := k.BondDenom(sdkCtx)
	if err != nil {
		return math.ZeroInt(), err
	}
	return k.bankKeeper.GetSupply(sdkCtx, bondDenom).Amount, nil
}

// BondedRatio the fraction of the staking tokens which are currently bonded
func (k Keeper) BondedRatio(ctx context.Context) (math.LegacyDec, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "BondedRatio")()

	stakeSupply, err := k.StakingTokenSupply(sdkCtx)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	if stakeSupply.IsPositive() {
		totalBonded, err := k.TotalBondedTokens(sdkCtx)
		if err != nil {
			return math.LegacyZeroDec(), err
		}
		return math.LegacyNewDecFromInt(totalBonded).QuoInt(stakeSupply), nil
	}

	return math.LegacyZeroDec(), nil
}
