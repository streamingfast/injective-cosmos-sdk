package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// get outstanding rewards
func (k Keeper) GetValidatorOutstandingRewardsCoins(ctx context.Context, val sdk.ValAddress) (meterResult sdk.DecCoins, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetValidatorOutstandingRewardsCoins")(&err)

	rewards, err := k.GetValidatorOutstandingRewards(sdkCtx, val)
	if err != nil {
		return nil, err
	}

	return rewards.Rewards, nil
}

// GetDistributionAccount returns the distribution ModuleAccount
func (k Keeper) GetDistributionAccount(ctx context.Context) sdk.ModuleAccountI {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetDistributionAccount")()

	return k.authKeeper.GetModuleAccount(sdkCtx, types.ModuleName)
}
