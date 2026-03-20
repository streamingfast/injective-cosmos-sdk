package keeper

import (
	"context"
	"errors"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.QueryServer = Keeper{}

// CurrentPlan implements the Query/CurrentPlan gRPC method
func (k Keeper) CurrentPlan(c context.Context, req *types.QueryCurrentPlanRequest) (*types.QueryCurrentPlanResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "CurrentPlan")()

	plan, err := k.GetUpgradePlan(sdkCtx)
	if err != nil {
		if errors.Is(err, types.ErrNoUpgradePlanFound) {
			return &types.QueryCurrentPlanResponse{}, nil
		}

		return nil, err
	}

	return &types.QueryCurrentPlanResponse{Plan: &plan}, nil
}

// AppliedPlan implements the Query/AppliedPlan gRPC method
func (k Keeper) AppliedPlan(c context.Context, req *types.QueryAppliedPlanRequest) (*types.QueryAppliedPlanResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "AppliedPlan")()

	applied, err := k.GetDoneHeight(sdkCtx, req.Name)

	return &types.QueryAppliedPlanResponse{Height: applied}, err
}

// UpgradedConsensusState implements the Query/UpgradedConsensusState gRPC method
func (k Keeper) UpgradedConsensusState(c context.Context, req *types.QueryUpgradedConsensusStateRequest) (*types.QueryUpgradedConsensusStateResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "UpgradedConsensusState")()

	//nolint:staticcheck // we're using a deprecated call for compatibility

	consState, err := k.GetUpgradedConsensusState(sdkCtx, req.LastHeight)
	if err != nil {
		if errors.Is(err, types.ErrNoUpgradedConsensusStateFound) {
			return &types.QueryUpgradedConsensusStateResponse{}, nil //nolint:staticcheck // we're using a deprecated call for compatibility
		}

		return nil, err
	}

	return &types.QueryUpgradedConsensusStateResponse{ //nolint:staticcheck // we're using a deprecated call for compatibility
		UpgradedConsensusState: consState,
	}, nil
}

// ModuleVersions implements the Query/QueryModuleVersions gRPC method
func (k Keeper) ModuleVersions(c context.Context, req *types.QueryModuleVersionsRequest) (*types.QueryModuleVersionsResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "ModuleVersions")()

	// check if a specific module was requested
	if len(req.ModuleName) > 0 {
		version, err := k.getModuleVersion(sdkCtx, req.ModuleName)
		if err != nil {
			// module requested, but not found or error happened
			return nil, errorsmod.Wrapf(err, "x/upgrade: QueryModuleVersions module %s not found", req.ModuleName)
		}

		// return the requested module
		res := []*types.ModuleVersion{{Name: req.ModuleName, Version: version}}
		return &types.QueryModuleVersionsResponse{ModuleVersions: res}, nil
	}

	// if no module requested return all module versions from state
	mv, err := k.GetModuleVersions(sdkCtx)
	if err != nil {
		return nil, err
	}

	return &types.QueryModuleVersionsResponse{
		ModuleVersions: mv,
	}, nil
}

// Authority implements the Query/Authority gRPC method, returning the account capable of performing upgrades
func (k Keeper) Authority(c context.Context, req *types.QueryAuthorityRequest) (*types.QueryAuthorityResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Authority")()

	return &types.QueryAuthorityResponse{Address: k.authority}, nil
}
