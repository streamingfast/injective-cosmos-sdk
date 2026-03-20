package keeper

import (
	"context"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TokensToConsensusPower converts input tokens to potential consensus-engine power
func (k Keeper) TokensToConsensusPower(ctx context.Context, tokens math.Int) int64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "TokensToConsensusPower")()

	return sdk.TokensToConsensusPower(tokens, k.PowerReduction(sdkCtx))
}

// TokensFromConsensusPower converts input power to tokens
func (k Keeper) TokensFromConsensusPower(ctx context.Context, power int64) math.Int {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "TokensFromConsensusPower")()

	return sdk.TokensFromConsensusPower(power, k.PowerReduction(sdkCtx))
}
