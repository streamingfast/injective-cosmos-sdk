package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// SetDelegationTransferReceiver adds a receiver address to the allowed receivers list for a delegator
func (k Keeper) SetDelegationTransferReceiver(ctx context.Context, receiverAddr sdk.AccAddress) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "SetDelegationTransferReceiver")()

	store := k.delegationTransferReceiversStore(sdkCtx)
	store.Set(receiverAddr.Bytes(), receiverAddr.Bytes())
}

// DeleteDelegationTransferReceiver removes a receiver address from the allowed receivers list for a delegator
func (k Keeper) DeleteDelegationTransferReceiver(ctx context.Context, receiverAddr sdk.AccAddress) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "DeleteDelegationTransferReceiver")()

	store := k.delegationTransferReceiversStore(sdkCtx)
	store.Delete(receiverAddr.Bytes())
}

// IsAllowedDelegationTransferReceiver checks if a receiver address is in the allowed receivers list for a delegator
func (k Keeper) IsAllowedDelegationTransferReceiver(ctx context.Context, receiverAddr sdk.AccAddress) bool {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "IsAllowedDelegationTransferReceiver")()

	store := k.delegationTransferReceiversStore(sdkCtx)
	return store.Has(receiverAddr.Bytes())
}

// GetAllAllowedDelegationTransferReceivers returns all allowed receiver addresses
func (k Keeper) GetAllAllowedDelegationTransferReceivers(ctx context.Context) []string {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetAllAllowedDelegationTransferReceivers")()

	store := k.delegationTransferReceiversStore(sdkCtx)
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	receivers := make([]string, 0)
	for ; iterator.Valid(); iterator.Next() {
		receivers = append(receivers, sdk.AccAddress(iterator.Value()).String())
	}
	return receivers
}

// delegationTransferReceiversStore returns a prefix store for delegation transfer receivers
func (k Keeper) delegationTransferReceiversStore(ctx context.Context) prefix.Store {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "delegationTransferReceiversStore")()

	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(sdkCtx))
	return prefix.NewStore(store, types.DelegationTransferReceiversKey)
}
