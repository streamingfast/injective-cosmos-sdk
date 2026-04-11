package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/x/mint/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.QueryServer = queryServer{}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{k}
}

type queryServer struct {
	k Keeper
}

// Params returns params of the mint module.
func (q queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (meterResult *types.QueryParamsResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer q.k.Meter(ctx).FuncTiming(&sdkCtx, "Params")(&err)

	params, err := q.k.Params.Get(sdkCtx)
	if err != nil {
		return nil, err
	}

	return &types.QueryParamsResponse{Params: params}, nil
}

// Inflation returns minter.Inflation of the mint module.
func (q queryServer) Inflation(ctx context.Context, _ *types.QueryInflationRequest) (meterResult *types.QueryInflationResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer q.k.Meter(ctx).FuncTiming(&sdkCtx, "Inflation")(&err)

	minter, err := q.k.Minter.Get(sdkCtx)
	if err != nil {
		return nil, err
	}

	return &types.QueryInflationResponse{Inflation: minter.Inflation}, nil
}

// AnnualProvisions returns minter.AnnualProvisions of the mint module.
func (q queryServer) AnnualProvisions(ctx context.Context, _ *types.QueryAnnualProvisionsRequest) (meterResult *types.QueryAnnualProvisionsResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer q.k.Meter(ctx).FuncTiming(&sdkCtx, "AnnualProvisions")(&err)

	minter, err := q.k.Minter.Get(sdkCtx)
	if err != nil {
		return nil, err
	}

	return &types.QueryAnnualProvisionsResponse{AnnualProvisions: minter.AnnualProvisions}, nil
}
