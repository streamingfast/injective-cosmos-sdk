package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"cosmossdk.io/x/nft"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/query"
)

var _ nft.QueryServer = Keeper{}

// Balance return the number of NFTs of a given class owned by the owner, same as balanceOf in ERC721
func (k Keeper) Balance(goCtx context.Context, r *nft.QueryBalanceRequest) (*nft.QueryBalanceResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Balance")()

	if r == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("empty request")
	}

	if len(r.ClassId) == 0 {
		return nil, nft.ErrEmptyClassID
	}

	owner, err := k.ac.StringToBytes(r.Owner)
	if err != nil {
		return nil, err
	}

	balance := k.GetBalance(sdkCtx, r.ClassId, owner)
	return &nft.QueryBalanceResponse{Amount: balance}, nil
}

// Owner return the owner of the NFT based on its class and id, same as ownerOf in ERC721
func (k Keeper) Owner(goCtx context.Context, r *nft.QueryOwnerRequest) (*nft.QueryOwnerResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Owner")()

	if r == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("empty request")
	}

	if len(r.ClassId) == 0 {
		return nil, nft.ErrEmptyClassID
	}

	if len(r.Id) == 0 {
		return nil, nft.ErrEmptyNFTID
	}

	owner := k.GetOwner(sdkCtx, r.ClassId, r.Id)
	if owner.Empty() {
		return &nft.QueryOwnerResponse{Owner: ""}, nil
	}
	ownerstr, err := k.ac.BytesToString(owner.Bytes())
	if err != nil {
		return nil, err
	}
	return &nft.QueryOwnerResponse{Owner: ownerstr}, nil
}

// Supply return the number of NFTs from the given class, same as totalSupply of ERC721.
func (k Keeper) Supply(goCtx context.Context, r *nft.QuerySupplyRequest) (*nft.QuerySupplyResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Supply")()

	if r == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("empty request")
	}

	if len(r.ClassId) == 0 {
		return nil, nft.ErrEmptyClassID
	}

	supply := k.GetTotalSupply(sdkCtx, r.ClassId)
	return &nft.QuerySupplyResponse{Amount: supply}, nil
}

// NFTs queries all NFTs of a given class or owner (at least one must be provided), similar to tokenByIndex in ERC721Enumerable
func (k Keeper) NFTs(goCtx context.Context, r *nft.QueryNFTsRequest) (*nft.QueryNFTsResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "NFTs")()

	if r == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("empty request")
	}

	var err error
	var owner sdk.AccAddress

	if len(r.Owner) > 0 {
		owner, err = k.ac.StringToBytes(r.Owner)
		if err != nil {
			return nil, err
		}
	}

	var nfts []*nft.NFT
	var pageRes *query.PageResponse

	switch {
	case len(r.ClassId) > 0 && len(r.Owner) > 0:
		if pageRes, err = query.Paginate(k.getClassStoreByOwner(sdkCtx, owner, r.ClassId), r.Pagination, func(key, _ []byte) error {
			nft, has := k.GetNFT(sdkCtx, r.ClassId, string(key))
			if has {
				nfts = append(nfts, &nft)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	case len(r.ClassId) > 0 && len(r.Owner) == 0:
		nftStore := k.getNFTStore(sdkCtx, r.ClassId)
		if pageRes, err = query.Paginate(nftStore, r.Pagination, func(_, value []byte) error {
			var nft nft.NFT
			if err := k.cdc.Unmarshal(value, &nft); err != nil {
				return err
			}
			nfts = append(nfts, &nft)
			return nil
		}); err != nil {
			return nil, err
		}
	case len(r.ClassId) == 0 && len(r.Owner) > 0:
		if pageRes, err = query.Paginate(k.prefixStoreNftOfClassByOwner(sdkCtx, owner), r.Pagination, func(key, value []byte) error {
			classID, nftID := parseNftOfClassByOwnerStoreKey(key)
			if n, has := k.GetNFT(sdkCtx, classID, nftID); has {
				nfts = append(nfts, &n)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	default:
		return nil, sdkerrors.ErrInvalidRequest.Wrap("must provide at least one of classID or owner")
	}
	return &nft.QueryNFTsResponse{
		Nfts:       nfts,
		Pagination: pageRes,
	}, nil
}

// NFT return an NFT based on its class and id.
func (k Keeper) NFT(goCtx context.Context, r *nft.QueryNFTRequest) (*nft.QueryNFTResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "NFT")()

	if r == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("empty request")
	}

	if len(r.ClassId) == 0 {
		return nil, nft.ErrEmptyClassID
	}
	if len(r.Id) == 0 {
		return nil, nft.ErrEmptyNFTID
	}

	n, has := k.GetNFT(sdkCtx, r.ClassId, r.Id)
	if !has {
		return nil, nft.ErrNFTNotExists.Wrapf("not found nft: class: %s, id: %s", r.ClassId, r.Id)
	}
	return &nft.QueryNFTResponse{Nft: &n}, nil
}

// Class return an NFT class based on its id
func (k Keeper) Class(goCtx context.Context, r *nft.QueryClassRequest) (*nft.QueryClassResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Class")()

	if r == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("empty request")
	}

	if len(r.ClassId) == 0 {
		return nil, nft.ErrEmptyClassID
	}

	class, has := k.GetClass(sdkCtx, r.ClassId)
	if !has {
		return nil, nft.ErrClassNotExists.Wrapf("not found class: %s", r.ClassId)
	}
	return &nft.QueryClassResponse{Class: &class}, nil
}

// Classes return all NFT classes
func (k Keeper) Classes(goCtx context.Context, r *nft.QueryClassesRequest) (*nft.QueryClassesResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Classes")()

	if r == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("empty request")
	}

	store := k.storeService.OpenKVStore(sdkCtx)
	classStore := prefix.NewStore(runtime.KVStoreAdapter(store), ClassKey)

	var classes []*nft.Class
	pageRes, err := query.Paginate(classStore, r.Pagination, func(_, value []byte) error {
		var class nft.Class
		if err := k.cdc.Unmarshal(value, &class); err != nil {
			return err
		}
		classes = append(classes, &class)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &nft.QueryClassesResponse{
		Classes:    classes,
		Pagination: pageRes,
	}, nil
}
