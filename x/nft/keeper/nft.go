package keeper

import (
	"context"

	"cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	"cosmossdk.io/x/nft"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Mint defines a method for minting a new nft
func (k Keeper) Mint(ctx context.Context, token nft.NFT, receiver sdk.AccAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Mint")()

	if !k.HasClass(sdkCtx, token.ClassId) {
		return errors.Wrap(nft.ErrClassNotExists, token.ClassId)
	}

	if k.HasNFT(sdkCtx, token.ClassId, token.Id) {
		return errors.Wrap(nft.ErrNFTExists, token.Id)
	}

	return k.mintWithNoCheck(sdkCtx, token, receiver)
}

// mintWithNoCheck defines a method for minting a new nft
// Note: this method does not check whether the class already exists in nft.
// The upper-layer application needs to check it when it needs to use it.
func (k Keeper) mintWithNoCheck(ctx context.Context, token nft.NFT, receiver sdk.AccAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "mintWithNoCheck")()

	k.setNFT(sdkCtx, token)
	k.setOwner(sdkCtx, token.ClassId, token.Id, receiver)
	k.incrTotalSupply(sdkCtx, token.ClassId)

	recStr, err := k.ac.BytesToString(receiver.Bytes())
	if err != nil {
		return err
	}
	return sdkCtx.EventManager().EmitTypedEvent(&nft.EventMint{
		ClassId: token.ClassId,
		Id:      token.Id,
		Owner:   recStr,
	})
}

// Burn defines a method for burning a nft from a specific account.
// Note: When the upper module uses this method, it needs to authenticate nft
func (k Keeper) Burn(ctx context.Context, classID, nftID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Burn")()

	if !k.HasClass(sdkCtx, classID) {
		return errors.Wrap(nft.ErrClassNotExists, classID)
	}

	if !k.HasNFT(sdkCtx, classID, nftID) {
		return errors.Wrap(nft.ErrNFTNotExists, nftID)
	}

	k.burnWithNoCheck(sdkCtx, classID, nftID)
	return nil
}

// burnWithNoCheck defines a method for burning a nft from a specific account.
// Note: this method does not check whether the class already exists in nft.
// The upper-layer application needs to check it when it needs to use it
func (k Keeper) burnWithNoCheck(ctx context.Context, classID, nftID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "burnWithNoCheck")()

	owner := k.GetOwner(sdkCtx, classID, nftID)
	nftStore := k.getNFTStore(sdkCtx, classID)
	nftStore.Delete([]byte(nftID))

	k.deleteOwner(sdkCtx, classID, nftID, owner)
	k.decrTotalSupply(sdkCtx, classID)
	ownerStr, err := k.ac.BytesToString(owner.Bytes())
	if err != nil {
		return err
	}

	return sdkCtx.EventManager().EmitTypedEvent(&nft.EventBurn{
		ClassId: classID,
		Id:      nftID,
		Owner:   ownerStr,
	})
}

// Update defines a method for updating an exist nft
// Note: When the upper module uses this method, it needs to authenticate nft
func (k Keeper) Update(ctx context.Context, token nft.NFT) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Update")()

	if !k.HasClass(sdkCtx, token.ClassId) {
		return errors.Wrap(nft.ErrClassNotExists, token.ClassId)
	}

	if !k.HasNFT(sdkCtx, token.ClassId, token.Id) {
		return errors.Wrap(nft.ErrNFTNotExists, token.Id)
	}
	k.updateWithNoCheck(sdkCtx, token)
	return nil
}

// Update defines a method for updating an exist nft
// Note: this method does not check whether the class already exists in nft.
// The upper-layer application needs to check it when it needs to use it
func (k Keeper) updateWithNoCheck(ctx context.Context, token nft.NFT) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "updateWithNoCheck")()

	k.setNFT(sdkCtx, token)
}

// Transfer defines a method for sending a nft from one account to another account.
// Note: When the upper module uses this method, it needs to authenticate nft
func (k Keeper) Transfer(ctx context.Context,
	classID string,
	nftID string,
	receiver sdk.AccAddress,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "Transfer")()

	if !k.HasClass(sdkCtx, classID) {
		return errors.Wrap(nft.ErrClassNotExists, classID)
	}

	if !k.HasNFT(sdkCtx, classID, nftID) {
		return errors.Wrap(nft.ErrNFTNotExists, nftID)
	}

	k.transferWithNoCheck(sdkCtx, classID, nftID, receiver)
	return nil
}

// Transfer defines a method for sending a nft from one account to another account.
// Note: this method does not check whether the class already exists in nft.
// The upper-layer application needs to check it when it needs to use it
func (k Keeper) transferWithNoCheck(ctx context.Context,
	classID string,
	nftID string,
	receiver sdk.AccAddress,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "transferWithNoCheck")()

	owner := k.GetOwner(sdkCtx, classID, nftID)
	k.deleteOwner(sdkCtx, classID, nftID, owner)
	k.setOwner(sdkCtx, classID, nftID, receiver)
	return nil
}

// GetNFT returns the nft information of the specified classID and nftID
func (k Keeper) GetNFT(ctx context.Context, classID, nftID string) (nft.NFT, bool) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetNFT")()

	store := k.getNFTStore(sdkCtx, classID)
	bz := store.Get([]byte(nftID))
	if len(bz) == 0 {
		return nft.NFT{}, false
	}
	var nft nft.NFT
	k.cdc.MustUnmarshal(bz, &nft)
	return nft, true
}

// GetNFTsOfClassByOwner returns all nft information of the specified classID under the specified owner
func (k Keeper) GetNFTsOfClassByOwner(ctx context.Context, classID string, owner sdk.AccAddress) (nfts []nft.NFT) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetNFTsOfClassByOwner")()

	ownerStore := k.getClassStoreByOwner(sdkCtx, owner, classID)
	iterator := ownerStore.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		nft, has := k.GetNFT(sdkCtx, classID, string(iterator.Key()))
		if has {
			nfts = append(nfts, nft)
		}
	}
	return nfts
}

// GetNFTsOfClass returns all nft information under the specified classID
func (k Keeper) GetNFTsOfClass(ctx context.Context, classID string) (nfts []nft.NFT) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetNFTsOfClass")()

	nftStore := k.getNFTStore(sdkCtx, classID)
	iterator := nftStore.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var nft nft.NFT
		k.cdc.MustUnmarshal(iterator.Value(), &nft)
		nfts = append(nfts, nft)
	}
	return nfts
}

// GetOwner returns the owner information of the specified nft
func (k Keeper) GetOwner(ctx context.Context, classID, nftID string) sdk.AccAddress {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetOwner")()

	store := k.storeService.OpenKVStore(sdkCtx)
	bz, err := store.Get(ownerStoreKey(classID, nftID))
	if err != nil {
		panic(err)
	}
	return sdk.AccAddress(bz)
}

// GetBalance returns the specified account, the number of all nfts under the specified classID
func (k Keeper) GetBalance(ctx context.Context, classID string, owner sdk.AccAddress) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetBalance")()

	nfts := k.GetNFTsOfClassByOwner(sdkCtx, classID, owner)
	return uint64(len(nfts))
}

// GetTotalSupply returns the number of all nfts under the specified classID
func (k Keeper) GetTotalSupply(ctx context.Context, classID string) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetTotalSupply")()

	store := k.storeService.OpenKVStore(sdkCtx)
	bz, err := store.Get(classTotalSupply(classID))
	if err != nil {
		panic(err)
	}
	return sdk.BigEndianToUint64(bz)
}

// HasNFT determines whether the specified classID and nftID exist
func (k Keeper) HasNFT(ctx context.Context, classID, id string) bool {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "HasNFT")()

	store := k.getNFTStore(sdkCtx, classID)
	return store.Has([]byte(id))
}

func (k Keeper) setNFT(ctx context.Context, token nft.NFT) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "setNFT")()

	nftStore := k.getNFTStore(sdkCtx, token.ClassId)
	bz := k.cdc.MustMarshal(&token)
	nftStore.Set([]byte(token.Id), bz)
}

func (k Keeper) setOwner(ctx context.Context, classID, nftID string, owner sdk.AccAddress) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "setOwner")()

	store := k.storeService.OpenKVStore(sdkCtx)
	store.Set(ownerStoreKey(classID, nftID), owner.Bytes())

	ownerStore := k.getClassStoreByOwner(sdkCtx, owner, classID)
	ownerStore.Set([]byte(nftID), Placeholder)
}

func (k Keeper) deleteOwner(ctx context.Context, classID, nftID string, owner sdk.AccAddress) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "deleteOwner")()

	store := k.storeService.OpenKVStore(sdkCtx)
	store.Delete(ownerStoreKey(classID, nftID))

	ownerStore := k.getClassStoreByOwner(sdkCtx, owner, classID)
	ownerStore.Delete([]byte(nftID))
}

func (k Keeper) getNFTStore(ctx context.Context, classID string) prefix.Store {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "getNFTStore")()

	store := k.storeService.OpenKVStore(sdkCtx)
	return prefix.NewStore(runtime.KVStoreAdapter(store), nftStoreKey(classID))
}

func (k Keeper) getClassStoreByOwner(ctx context.Context, owner sdk.AccAddress, classID string) prefix.Store {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "getClassStoreByOwner")()

	store := k.storeService.OpenKVStore(sdkCtx)
	key := nftOfClassByOwnerStoreKey(owner, classID)
	return prefix.NewStore(runtime.KVStoreAdapter(store), key)
}

func (k Keeper) prefixStoreNftOfClassByOwner(ctx context.Context, owner sdk.AccAddress) prefix.Store {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "prefixStoreNftOfClassByOwner")()

	store := k.storeService.OpenKVStore(sdkCtx)
	key := prefixNftOfClassByOwnerStoreKey(owner)
	return prefix.NewStore(runtime.KVStoreAdapter(store), key)
}

func (k Keeper) incrTotalSupply(ctx context.Context, classID string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "incrTotalSupply")()

	supply := k.GetTotalSupply(sdkCtx, classID) + 1
	k.updateTotalSupply(sdkCtx, classID, supply)
}

func (k Keeper) decrTotalSupply(ctx context.Context, classID string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "decrTotalSupply")()

	supply := k.GetTotalSupply(sdkCtx, classID) - 1
	k.updateTotalSupply(sdkCtx, classID, supply)
}

func (k Keeper) updateTotalSupply(ctx context.Context, classID string, supply uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "updateTotalSupply")()

	store := k.storeService.OpenKVStore(sdkCtx)
	supplyKey := classTotalSupply(classID)
	err := store.Set(supplyKey, sdk.Uint64ToBigEndian(supply))
	if err != nil {
		panic(err)
	}
}
