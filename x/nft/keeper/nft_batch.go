package keeper

import (
	"context"

	"cosmossdk.io/errors"
	"cosmossdk.io/x/nft"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BatchMint defines a method for minting a batch of nfts
func (k Keeper) BatchMint(ctx context.Context,
	tokens []nft.NFT,
	receiver sdk.AccAddress,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "BatchMint")()

	checked := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		if !checked[token.ClassId] && !k.HasClass(sdkCtx, token.ClassId) {
			return errors.Wrap(nft.ErrClassNotExists, token.ClassId)
		}

		if k.HasNFT(sdkCtx, token.ClassId, token.Id) {
			return errors.Wrap(nft.ErrNFTExists, token.Id)
		}

		checked[token.ClassId] = true
		if err := k.mintWithNoCheck(sdkCtx, token, receiver); err != nil {
			return err
		}
	}
	return nil
}

// BatchBurn defines a method for burning a batch of nfts from a specific classID.
// Note: When the upper module uses this method, it needs to authenticate nft
func (k Keeper) BatchBurn(ctx context.Context, classID string, nftIDs []string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "BatchBurn")()

	if !k.HasClass(sdkCtx, classID) {
		return errors.Wrap(nft.ErrClassNotExists, classID)
	}
	for _, nftID := range nftIDs {
		if !k.HasNFT(sdkCtx, classID, nftID) {
			return errors.Wrap(nft.ErrNFTNotExists, nftID)
		}
		if err := k.burnWithNoCheck(sdkCtx, classID, nftID); err != nil {
			return err
		}
	}
	return nil
}

// BatchUpdate defines a method for updating a batch of exist nfts
// Note: When the upper module uses this method, it needs to authenticate nft
func (k Keeper) BatchUpdate(ctx context.Context, tokens []nft.NFT) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "BatchUpdate")()

	checked := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		if !checked[token.ClassId] && !k.HasClass(sdkCtx, token.ClassId) {
			return errors.Wrap(nft.ErrClassNotExists, token.ClassId)
		}

		if !k.HasNFT(sdkCtx, token.ClassId, token.Id) {
			return errors.Wrap(nft.ErrNFTNotExists, token.Id)
		}
		checked[token.ClassId] = true
		k.updateWithNoCheck(sdkCtx, token)
	}
	return nil
}

// BatchTransfer defines a method for sending a batch of nfts from one account to another account from a specific classID.
// Note: When the upper module uses this method, it needs to authenticate nft
func (k Keeper) BatchTransfer(ctx context.Context,
	classID string,
	nftIDs []string,
	receiver sdk.AccAddress,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "BatchTransfer")()

	if !k.HasClass(sdkCtx, classID) {
		return errors.Wrap(nft.ErrClassNotExists, classID)
	}
	for _, nftID := range nftIDs {
		if !k.HasNFT(sdkCtx, classID, nftID) {
			return errors.Wrap(nft.ErrNFTNotExists, nftID)
		}
		if err := k.transferWithNoCheck(sdkCtx, classID, nftID, receiver); err != nil {
			return errors.Wrap(nft.ErrNFTNotExists, nftID)
		}
	}
	return nil
}
