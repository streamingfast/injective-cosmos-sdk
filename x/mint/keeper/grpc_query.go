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
func (q queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer q.k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Params")()

	params, err := q.k.Params.Get(sdkCtx)
	if err != nil {
		return nil, err
	}

	return &types.QueryParamsResponse{Params: params}, nil
}

// Inflation returns minter.Inflation of the mint module.
func (q queryServer) Inflation(ctx context.Context, _ *types.QueryInflationRequest) (*types.QueryInflationResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer q.k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Inflation")()

	minter, err := q.k.Minter.Get(sdkCtx)
	if err != nil {
		return nil, err
	}

	return &types.QueryInflationResponse{Inflation: minter.Inflation}, nil
}

// AnnualProvisions returns minter.AnnualProvisions of the mint module.
func (q queryServer) AnnualProvisions(ctx context.Context, _ *types.QueryAnnualProvisionsRequest) (*types.QueryAnnualProvisionsResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer q.k.Meter(sdkCtx).FuncTiming(&sdkCtx, "AnnualProvisions")()

	minter, err := q.k.Minter.Get(sdkCtx)
	if err != nil {
		return nil, err
	}

	return &types.QueryAnnualProvisionsResponse{AnnualProvisions: minter.AnnualProvisions}, nil
}
