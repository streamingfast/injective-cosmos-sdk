package module

import (
	"context"

	"cosmossdk.io/x/feegrant/keeper"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func EndBlocker(ctx context.Context, k keeper.Keeper) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "EndBlocker")(&err)
	// 200 is an arbitrary value, we can change it later if needed
	return k.RemoveExpiredAllowances(ctx, 200)
}
