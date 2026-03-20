package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
)

type Querier struct {
	BaseKeeper
}

var _ types.QueryServer = BaseKeeper{}

func NewQuerier(keeper *BaseKeeper) Querier {
	return Querier{BaseKeeper: *keeper}
}

// Balance implements the Query/Balance gRPC method
func (k BaseKeeper) Balance(ctx context.Context, req *types.QueryBalanceRequest) (*types.QueryBalanceResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Balance")()

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := sdk.ValidateDenom(req.Denom); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	address, err := k.ak.AddressCodec().StringToBytes(req.Address)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %s", err.Error())
	}

	balance := k.GetBalance(sdkCtx, address, req.Denom)

	return &types.QueryBalanceResponse{Balance: &balance}, nil
}

// AllBalances implements the Query/AllBalances gRPC method
func (k BaseKeeper) AllBalances(ctx context.Context, req *types.QueryAllBalancesRequest) (*types.QueryAllBalancesResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "AllBalances")()

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	addr, err := k.ak.AddressCodec().StringToBytes(req.Address)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %s", err.Error())
	}

	balances, pageRes, err := query.CollectionPaginate(
		sdkCtx,
		k.Balances,
		req.Pagination,
		func(key collections.Pair[sdk.AccAddress, string], value math.Int) (sdk.Coin, error) {
			if req.ResolveDenom {
				if metadata, ok := k.GetDenomMetaData(sdkCtx, key.K2()); ok {
					return sdk.NewCoin(metadata.Display, value), nil
				}
			}
			return sdk.NewCoin(key.K2(), value), nil
		},
		query.WithCollectionPaginationPairPrefix[sdk.AccAddress, string](addr),
	)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "paginate: %v", err)
	}

	return &types.QueryAllBalancesResponse{Balances: balances, Pagination: pageRes}, nil
}

// SpendableBalances implements a gRPC query handler for retrieving an account's
// spendable balances.
func (k BaseKeeper) SpendableBalances(ctx context.Context, req *types.QuerySpendableBalancesRequest) (*types.QuerySpendableBalancesResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "SpendableBalances")()

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	addr, err := k.ak.AddressCodec().StringToBytes(req.Address)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %s", err.Error())
	}

	zeroAmt := math.ZeroInt()

	balances, pageRes, err := query.CollectionPaginate(sdkCtx, k.Balances, req.Pagination, func(key collections.Pair[sdk.AccAddress, string], _ math.Int) (coin sdk.Coin, err error) {
		return sdk.NewCoin(key.K2(), zeroAmt), nil
	}, query.WithCollectionPaginationPairPrefix[sdk.AccAddress, string](addr))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "paginate: %v", err)
	}

	result := sdk.NewCoins()
	spendable := k.SpendableCoins(sdkCtx, addr)

	for _, c := range balances {
		result = append(result, sdk.NewCoin(c.Denom, spendable.AmountOf(c.Denom)))
	}

	return &types.QuerySpendableBalancesResponse{Balances: result, Pagination: pageRes}, nil
}

// SpendableBalanceByDenom implements a gRPC query handler for retrieving an account's
// spendable balance for a specific denom.
func (k BaseKeeper) SpendableBalanceByDenom(ctx context.Context, req *types.QuerySpendableBalanceByDenomRequest) (*types.QuerySpendableBalanceByDenomResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "SpendableBalanceByDenom")()

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	addr, err := k.ak.AddressCodec().StringToBytes(req.Address)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %s", err.Error())
	}

	if err := sdk.ValidateDenom(req.Denom); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	spendable := k.SpendableCoin(sdkCtx, addr, req.Denom)

	return &types.QuerySpendableBalanceByDenomResponse{Balance: &spendable}, nil
}

// TotalSupply implements the Query/TotalSupply gRPC method
func (k BaseKeeper) TotalSupply(ctx context.Context, req *types.QueryTotalSupplyRequest) (*types.QueryTotalSupplyResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "TotalSupply")()

	totalSupply, pageRes, err := k.GetPaginatedTotalSupply(sdkCtx, req.Pagination)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryTotalSupplyResponse{Supply: totalSupply, Pagination: pageRes}, nil
}

// SupplyOf implements the Query/SupplyOf gRPC method
func (k BaseKeeper) SupplyOf(c context.Context, req *types.QuerySupplyOfRequest) (*types.QuerySupplyOfResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "SupplyOf")()

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := sdk.ValidateDenom(req.Denom); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	supply := k.GetSupply(sdkCtx, req.Denom)

	return &types.QuerySupplyOfResponse{Amount: sdk.NewCoin(req.Denom, supply.Amount)}, nil
}

// Params implements the gRPC service handler for querying x/bank parameters.
func (k BaseKeeper) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Params")()

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	params := k.GetParams(sdkCtx)

	return &types.QueryParamsResponse{Params: params}, nil
}

// DenomsMetadata implements Query/DenomsMetadata gRPC method.
func (k BaseKeeper) DenomsMetadata(c context.Context, req *types.QueryDenomsMetadataRequest) (*types.QueryDenomsMetadataResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "DenomsMetadata")()

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}
	kvStore := runtime.KVStoreAdapter(k.storeService.OpenKVStore(sdkCtx))
	store := prefix.NewStore(kvStore, types.DenomMetadataPrefix)

	metadatas := []types.Metadata{}
	pageRes, err := query.Paginate(store, req.Pagination, func(_, value []byte) error {
		var metadata types.Metadata
		k.cdc.MustUnmarshal(value, &metadata)

		metadatas = append(metadatas, metadata)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryDenomsMetadataResponse{
		Metadatas:  metadatas,
		Pagination: pageRes,
	}, nil
}

// DenomMetadata implements Query/DenomMetadata gRPC method.
func (k BaseKeeper) DenomMetadata(c context.Context, req *types.QueryDenomMetadataRequest) (*types.QueryDenomMetadataResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "DenomMetadata")()

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	if err := sdk.ValidateDenom(req.Denom); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	metadata, found := k.GetDenomMetaData(sdkCtx, req.Denom)
	if !found {
		return nil, status.Errorf(codes.NotFound, "client metadata for denom %s", req.Denom)
	}

	return &types.QueryDenomMetadataResponse{
		Metadata: metadata,
	}, nil
}

// DenomMetadataByQueryString is identical to DenomMetadata query, but receives request via query string.
func (k BaseKeeper) DenomMetadataByQueryString(c context.Context, req *types.QueryDenomMetadataByQueryStringRequest) (*types.QueryDenomMetadataByQueryStringResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(c)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "DenomMetadataByQueryString")()

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	res, err := k.DenomMetadata(sdkCtx, &types.QueryDenomMetadataRequest{
		Denom: req.Denom,
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryDenomMetadataByQueryStringResponse{Metadata: res.Metadata}, nil
}

func (k BaseKeeper) DenomOwners(
	ctx context.Context,
	req *types.QueryDenomOwnersRequest,
) (*types.QueryDenomOwnersResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "DenomOwners")()

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	if err := sdk.ValidateDenom(req.Denom); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	denomOwners, pageRes, err := query.CollectionPaginate(
		sdkCtx,
		k.Balances.Indexes.Denom,
		req.Pagination,
		func(key collections.Pair[string, sdk.AccAddress], value collections.NoValue) (*types.DenomOwner, error) {
			amt, err := k.Balances.Get(sdkCtx, collections.Join(key.K2(), req.Denom))
			if err != nil {
				return nil, err
			}
			return &types.DenomOwner{Address: key.K2().String(), Balance: sdk.NewCoin(req.Denom, amt)}, nil
		},
		query.WithCollectionPaginationPairPrefix[string, sdk.AccAddress](req.Denom),
	)
	if err != nil {
		return nil, err
	}

	return &types.QueryDenomOwnersResponse{DenomOwners: denomOwners, Pagination: pageRes}, nil
}

func (k BaseKeeper) SendEnabled(goCtx context.Context, req *types.QuerySendEnabledRequest) (*types.QuerySendEnabledResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "SendEnabled")()

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	resp := &types.QuerySendEnabledResponse{}
	if len(req.Denoms) > 0 {
		for _, denom := range req.Denoms {
			if se, ok := k.getSendEnabled(sdkCtx, denom); ok {
				resp.SendEnabled = append(resp.SendEnabled, types.NewSendEnabled(denom, se))
			}
		}
	} else {
		results, pageResp, err := query.CollectionPaginate(
			sdkCtx,
			k.BaseViewKeeper.SendEnabled,
			req.Pagination, func(key string, value bool) (*types.SendEnabled, error) {
				return types.NewSendEnabled(key, value), nil
			},
		)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		resp.SendEnabled = results
		resp.Pagination = pageResp
	}

	return resp, nil
}

// DenomOwnersByQuery is identical to DenomOwner query, but receives denom values via query string.
func (k BaseKeeper) DenomOwnersByQuery(ctx context.Context, req *types.QueryDenomOwnersByQueryRequest) (*types.QueryDenomOwnersByQueryResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "DenomOwnersByQuery")()

	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}
	resp, err := k.DenomOwners(sdkCtx, &types.QueryDenomOwnersRequest{
		Denom:      req.Denom,
		Pagination: req.Pagination,
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryDenomOwnersByQueryResponse{DenomOwners: resp.DenomOwners, Pagination: resp.Pagination}, nil
}
