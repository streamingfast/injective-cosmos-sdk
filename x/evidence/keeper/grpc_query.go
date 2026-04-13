package keeper

import (
	"context"
	"encoding/hex"
	"fmt"

	proto "github.com/cosmos/gogoproto/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cosmossdk.io/x/evidence/exported"
	"cosmossdk.io/x/evidence/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

var _ types.QueryServer = Querier{}

type Querier struct {
	k *Keeper
}

func NewQuerier(keeper *Keeper) Querier {
	return Querier{k: keeper}
}

// Evidence implements the Query/Evidence gRPC method
func (k Querier) Evidence(c context.Context, req *types.QueryEvidenceRequest) (meterResult *types.QueryEvidenceResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.k.Meter(c).FuncTiming(&sdkCtx, "Evidence")(&err)

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	if req.Hash == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request; hash is empty")
	}

	decodedHash, err := hex.DecodeString(req.Hash)
	if err != nil {
		return nil, fmt.Errorf("invalid evidence hash: %w", err)
	}

	evidence, _ := k.k.Evidences.Get(sdkCtx, decodedHash)
	if evidence == nil {
		return nil, status.Errorf(codes.NotFound, "evidence %s not found", req.Hash)
	}

	msg, ok := evidence.(proto.Message)
	if !ok {
		return nil, status.Errorf(codes.Internal, "can't protomarshal %T", msg)
	}

	evidenceAny, err := codectypes.NewAnyWithValue(msg)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryEvidenceResponse{Evidence: evidenceAny}, nil
}

// AllEvidence implements the Query/AllEvidence gRPC method
func (k Querier) AllEvidence(ctx context.Context, req *types.QueryAllEvidenceRequest) (meterResult *types.QueryAllEvidenceResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.k.Meter(ctx).FuncTiming(&sdkCtx, "AllEvidence")(&err)

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	evidences, pageRes, err := query.CollectionPaginate(sdkCtx, k.k.Evidences, req.Pagination, func(_ []byte, value exported.Evidence) (*codectypes.Any, error) {
		return codectypes.NewAnyWithValue(value)
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryAllEvidenceResponse{Evidence: evidences, Pagination: pageRes}, nil
}
