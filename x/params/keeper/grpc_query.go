package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/params/types/proposal"
)

var _ proposal.QueryServer = Keeper{}

// Params returns subspace params
func (k Keeper) Params(c context.Context, req *proposal.QueryParamsRequest) (meterResult *proposal.QueryParamsResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(c).FuncTiming(&sdkCtx, "Params")(&err)

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	if req.Subspace == "" || req.Key == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request")
	}

	ss, ok := k.GetSubspace(req.Subspace)
	if !ok {
		return nil, errors.Wrap(proposal.ErrUnknownSubspace, req.Subspace)
	}

	rawValue := ss.GetRaw(sdkCtx, []byte(req.Key))
	param := proposal.NewParamChange(req.Subspace, req.Key, string(rawValue))

	return &proposal.QueryParamsResponse{Param: param}, nil
}

// Subspaces implements the gRPC query handler for fetching all registered
// subspaces and all the keys for each subspace.
func (k Keeper) Subspaces(
	goCtx context.Context,
	req *proposal.QuerySubspacesRequest,
) (meterResult *proposal.QuerySubspacesResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(goCtx).FuncTiming(&sdkCtx, "Subspaces")(&err)

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	spaces := k.GetSubspaces()
	resp := &proposal.QuerySubspacesResponse{
		Subspaces: make([]*proposal.Subspace, len(spaces)),
	}

	for i, ss := range spaces {
		var keys []string
		ss.IterateKeys(sdkCtx, func(key []byte) bool {
			keys = append(keys, string(key))
			return false
		})

		resp.Subspaces[i] = &proposal.Subspace{
			Subspace: ss.Name(),
			Keys:     keys,
		}
	}

	return resp, nil
}
