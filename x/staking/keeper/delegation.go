package keeper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	corestore "cosmossdk.io/core/store"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GetDelegation returns a specific delegation.
func (k Keeper) GetDelegation(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (meterResult types.Delegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetDelegation")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	key := types.GetDelegationKey(delAddr, valAddr)

	value, err := store.Get(key)
	if err != nil {
		return types.Delegation{}, err
	}

	if value == nil {
		return types.Delegation{}, types.ErrNoDelegation
	}

	return types.UnmarshalDelegation(k.cdc, value)
}

// IterateAllDelegations iterates through all of the delegations.
func (k Keeper) IterateAllDelegations(ctx context.Context, cb func(delegation types.Delegation) (stop bool)) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "IterateAllDelegations")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	iterator, err := store.Iterator(types.DelegationKey, storetypes.PrefixEndBytes(types.DelegationKey))
	if err != nil {
		return err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		delegation := types.MustUnmarshalDelegation(k.cdc, iterator.Value())
		if cb(delegation) {
			break
		}
	}

	return nil
}

// GetAllDelegations returns all delegations used during genesis dump.
func (k Keeper) GetAllDelegations(ctx context.Context) (delegations []types.Delegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetAllDelegations")(&err)

	err = k.IterateAllDelegations(sdkCtx, func(delegation types.Delegation) bool {
		delegations = append(delegations, delegation)
		return false
	})

	return delegations, err
}

// GetValidatorDelegations returns all delegations to a specific validator.
// Useful for querier.
func (k Keeper) GetValidatorDelegations(ctx context.Context, valAddr sdk.ValAddress) (delegations []types.Delegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetValidatorDelegations")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	prefix := types.GetDelegationsByValPrefixKey(valAddr)
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return delegations, err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var delegation types.Delegation
		valAddr, delAddr, err := types.ParseDelegationsByValKey(iterator.Key())
		if err != nil {
			return delegations, err
		}

		bz, err := store.Get(types.GetDelegationKey(delAddr, valAddr))
		if err != nil {
			return delegations, err
		}

		if err = k.cdc.Unmarshal(bz, &delegation); err != nil {
			return delegations, err
		}

		delegations = append(delegations, delegation)
	}

	return delegations, nil
}

// GetDelegatorDelegations returns a given amount of all the delegations from a
// delegator.
func (k Keeper) GetDelegatorDelegations(ctx context.Context, delegator sdk.AccAddress, maxRetrieve uint16) (delegations []types.Delegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetDelegatorDelegations")(&err)

	delegations = make([]types.Delegation, maxRetrieve)
	store := k.storeService.OpenKVStore(sdkCtx)
	delegatorPrefixKey := types.GetDelegationsKey(delegator)

	iterator, err := store.Iterator(delegatorPrefixKey, storetypes.PrefixEndBytes(delegatorPrefixKey))
	if err != nil {
		return delegations, err
	}
	defer iterator.Close()

	i := 0
	for ; iterator.Valid() && i < int(maxRetrieve); iterator.Next() {
		delegation, err := types.UnmarshalDelegation(k.cdc, iterator.Value())
		if err != nil {
			return delegations, err
		}
		delegations[i] = delegation
		i++
	}

	return delegations[:i], nil // trim if the array length < maxRetrieve
}

// SetDelegation sets a delegation.
func (k Keeper) SetDelegation(ctx context.Context, delegation types.Delegation) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "SetDelegation")(&err)

	delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(delegation.DelegatorAddress)
	if err != nil {
		return err
	}

	valAddr, err := k.validatorAddressCodec.StringToBytes(delegation.GetValidatorAddr())
	if err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(sdkCtx)
	b := types.MustMarshalDelegation(k.cdc, delegation)
	err = store.Set(types.GetDelegationKey(delegatorAddress, valAddr), b)
	if err != nil {
		return err
	}

	// set the delegation in validator delegator index
	return store.Set(types.GetDelegationsByValKey(valAddr, delegatorAddress), []byte{})
}

// RemoveDelegation removes a delegation
func (k Keeper) RemoveDelegation(ctx context.Context, delegation types.Delegation) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "RemoveDelegation")(&err)

	delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(delegation.DelegatorAddress)
	if err != nil {
		return err
	}

	valAddr, err := k.validatorAddressCodec.StringToBytes(delegation.GetValidatorAddr())
	if err != nil {
		return err
	}

	// TODO: Consider calling hooks outside of the store wrapper functions, it's unobvious.
	if err = k.Hooks().BeforeDelegationRemoved(sdkCtx, delegatorAddress, valAddr); err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(sdkCtx)
	err = store.Delete(types.GetDelegationKey(delegatorAddress, valAddr))
	if err != nil {
		return err
	}

	return store.Delete(types.GetDelegationsByValKey(valAddr, delegatorAddress))
}

// GetUnbondingDelegations returns a given amount of all the delegator unbonding-delegations.
func (k Keeper) GetUnbondingDelegations(ctx context.Context, delegator sdk.AccAddress, maxRetrieve uint16) (unbondingDelegations []types.UnbondingDelegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetUnbondingDelegations")(&err)

	unbondingDelegations = make([]types.UnbondingDelegation, maxRetrieve)

	store := k.storeService.OpenKVStore(sdkCtx)
	delegatorPrefixKey := types.GetUBDsKey(delegator)

	iterator, err := store.Iterator(delegatorPrefixKey, storetypes.PrefixEndBytes(delegatorPrefixKey))
	if err != nil {
		return unbondingDelegations, err
	}
	defer iterator.Close()

	i := 0
	for ; iterator.Valid() && i < int(maxRetrieve); iterator.Next() {
		unbondingDelegation, err := types.UnmarshalUBD(k.cdc, iterator.Value())
		if err != nil {
			return unbondingDelegations, err
		}
		unbondingDelegations[i] = unbondingDelegation
		i++
	}

	return unbondingDelegations[:i], nil // trim if the array length < maxRetrieve
}

// GetUnbondingDelegation returns a unbonding delegation.
func (k Keeper) GetUnbondingDelegation(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (ubd types.UnbondingDelegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetUnbondingDelegation")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	key := types.GetUBDKey(delAddr, valAddr)
	value, err := store.Get(key)
	if err != nil {
		return ubd, err
	}

	if value == nil {
		return ubd, types.ErrNoUnbondingDelegation
	}

	return types.UnmarshalUBD(k.cdc, value)
}

// GetUnbondingDelegationsFromValidator returns all unbonding delegations from a
// particular validator.
func (k Keeper) GetUnbondingDelegationsFromValidator(ctx context.Context, valAddr sdk.ValAddress) (ubds []types.UnbondingDelegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetUnbondingDelegationsFromValidator")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	prefix := types.GetUBDsByValIndexKey(valAddr)
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return ubds, err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		key := types.GetUBDKeyFromValIndexKey(iterator.Key())
		value, err := store.Get(key)
		if err != nil {
			return ubds, err
		}
		ubd, err := types.UnmarshalUBD(k.cdc, value)
		if err != nil {
			return ubds, err
		}
		ubds = append(ubds, ubd)
	}

	return ubds, nil
}

// IterateUnbondingDelegations iterates through all of the unbonding delegations.
func (k Keeper) IterateUnbondingDelegations(ctx context.Context, fn func(index int64, ubd types.UnbondingDelegation) (stop bool)) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "IterateUnbondingDelegations")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	prefix := types.UnbondingDelegationKey
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	defer iterator.Close()

	for i := int64(0); iterator.Valid(); iterator.Next() {
		ubd, err := types.UnmarshalUBD(k.cdc, iterator.Value())
		if err != nil {
			return err
		}
		if stop := fn(i, ubd); stop {
			break
		}
		i++
	}

	return nil
}

// GetDelegatorUnbonding returns the total amount a delegator has unbonding.
func (k Keeper) GetDelegatorUnbonding(ctx context.Context, delegator sdk.AccAddress) (meterResult math.Int, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetDelegatorUnbonding")(&err)

	unbonding := math.ZeroInt()
	err = k.IterateDelegatorUnbondingDelegations(sdkCtx, delegator, func(ubd types.UnbondingDelegation) bool {
		for _, entry := range ubd.Entries {
			unbonding = unbonding.Add(entry.Balance)
		}
		return false
	})
	return unbonding, err
}

// IterateDelegatorUnbondingDelegations iterates through a delegator's unbonding delegations.
func (k Keeper) IterateDelegatorUnbondingDelegations(ctx context.Context, delegator sdk.AccAddress, cb func(ubd types.UnbondingDelegation) (stop bool)) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "IterateDelegatorUnbondingDelegations")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	prefix := types.GetUBDsKey(delegator)
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		ubd, err := types.UnmarshalUBD(k.cdc, iterator.Value())
		if err != nil {
			return err
		}
		if cb(ubd) {
			break
		}
	}

	return nil
}

// GetDelegatorBonded returs the total amount a delegator has bonded.
func (k Keeper) GetDelegatorBonded(ctx context.Context, delegator sdk.AccAddress) (meterResult math.Int, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetDelegatorBonded")(&err)

	bonded := math.LegacyZeroDec()

	err = k.IterateDelegatorDelegations(sdkCtx, delegator, func(delegation types.Delegation) bool {
		validatorAddr, err := k.validatorAddressCodec.StringToBytes(delegation.ValidatorAddress)
		if err != nil {
			panic(err) // shouldn't happen
		}
		validator, err := k.GetValidator(sdkCtx, validatorAddr)
		if err == nil {
			shares := delegation.Shares
			tokens := validator.TokensFromSharesTruncated(shares)
			bonded = bonded.Add(tokens)
		}
		return false
	})
	return bonded.RoundInt(), err
}

// IterateDelegatorDelegations iterates through one delegator's delegations.
func (k Keeper) IterateDelegatorDelegations(ctx context.Context, delegator sdk.AccAddress, cb func(delegation types.Delegation) (stop bool)) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "IterateDelegatorDelegations")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	prefix := types.GetDelegationsKey(delegator)
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		delegation, err := types.UnmarshalDelegation(k.cdc, iterator.Value())
		if err != nil {
			return err
		}
		if cb(delegation) {
			break
		}
	}
	return nil
}

// IterateDelegatorRedelegations iterates through one delegator's redelegations.
func (k Keeper) IterateDelegatorRedelegations(ctx context.Context, delegator sdk.AccAddress, cb func(red types.Redelegation) (stop bool)) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "IterateDelegatorRedelegations")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	delegatorPrefixKey := types.GetREDsKey(delegator)
	iterator, err := store.Iterator(delegatorPrefixKey, storetypes.PrefixEndBytes(delegatorPrefixKey))
	if err != nil {
		return err
	}

	for ; iterator.Valid(); iterator.Next() {
		red, err := types.UnmarshalRED(k.cdc, iterator.Value())
		if err != nil {
			return err
		}
		if cb(red) {
			break
		}
	}
	return nil
}

// HasMaxUnbondingDelegationEntries checks if unbonding delegation has maximum number of entries.
func (k Keeper) HasMaxUnbondingDelegationEntries(ctx context.Context, delegatorAddr sdk.AccAddress, validatorAddr sdk.ValAddress) (meterResult bool, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "HasMaxUnbondingDelegationEntries")(&err)

	ubd, err := k.GetUnbondingDelegation(sdkCtx, delegatorAddr, validatorAddr)
	if err != nil && !errors.Is(err, types.ErrNoUnbondingDelegation) {
		return false, err
	}

	maxEntries, err := k.MaxEntries(sdkCtx)
	if err != nil {
		return false, err
	}
	return len(ubd.Entries) >= int(maxEntries), nil
}

// SetUnbondingDelegation sets the unbonding delegation and associated index.
func (k Keeper) SetUnbondingDelegation(ctx context.Context, ubd types.UnbondingDelegation) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "SetUnbondingDelegation")(&err)

	delAddr, err := k.authKeeper.AddressCodec().StringToBytes(ubd.DelegatorAddress)
	if err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(sdkCtx)
	bz := types.MustMarshalUBD(k.cdc, ubd)
	valAddr, err := k.validatorAddressCodec.StringToBytes(ubd.ValidatorAddress)
	if err != nil {
		return err
	}
	key := types.GetUBDKey(delAddr, valAddr)
	err = store.Set(key, bz)
	if err != nil {
		return err
	}

	return store.Set(types.GetUBDByValIndexKey(delAddr, valAddr), []byte{}) // index, store empty bytes
}

// RemoveUnbondingDelegation removes the unbonding delegation object and associated index.
func (k Keeper) RemoveUnbondingDelegation(ctx context.Context, ubd types.UnbondingDelegation) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "RemoveUnbondingDelegation")(&err)

	delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(ubd.DelegatorAddress)
	if err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(sdkCtx)
	addr, err := k.validatorAddressCodec.StringToBytes(ubd.ValidatorAddress)
	if err != nil {
		return err
	}
	key := types.GetUBDKey(delegatorAddress, addr)
	err = store.Delete(key)
	if err != nil {
		return err
	}

	return store.Delete(types.GetUBDByValIndexKey(delegatorAddress, addr))
}

// SetUnbondingDelegationEntry adds an entry to the unbonding delegation at
// the given addresses. It creates the unbonding delegation if it does not exist.
func (k Keeper) SetUnbondingDelegationEntry(
	ctx context.Context, delegatorAddr sdk.AccAddress, validatorAddr sdk.ValAddress,
	creationHeight int64, minTime time.Time, balance math.Int,
) (meterResult types.UnbondingDelegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "SetUnbondingDelegationEntry")(&err)

	id, err := k.IncrementUnbondingID(sdkCtx)
	if err != nil {
		return types.UnbondingDelegation{}, err
	}

	isNewUbdEntry := true
	ubd, err := k.GetUnbondingDelegation(sdkCtx, delegatorAddr, validatorAddr)
	if err == nil {
		isNewUbdEntry = ubd.AddEntry(creationHeight, minTime, balance, id)
	} else if errors.Is(err, types.ErrNoUnbondingDelegation) {
		ubd = types.NewUnbondingDelegation(delegatorAddr, validatorAddr, creationHeight, minTime, balance, id, k.validatorAddressCodec, k.authKeeper.AddressCodec())
	} else {
		return ubd, err
	}

	if err = k.SetUnbondingDelegation(sdkCtx, ubd); err != nil {
		return ubd, err
	}

	// only call the hook for new entries since
	// calls to AfterUnbondingInitiated are not idempotent
	if isNewUbdEntry {
		// Add to the UBDByUnbondingOp index to look up the UBD by the UBDE ID
		if err = k.SetUnbondingDelegationByUnbondingID(sdkCtx, ubd, id); err != nil {
			return ubd, err
		}

		if err = k.Hooks().AfterUnbondingInitiated(sdkCtx, id); err != nil {
			k.Logger(sdkCtx).Error("failed to call after unbonding initiated hook", "error", err)
		}
	}
	return ubd, nil
}

// unbonding delegation queue timeslice operations

// GetUBDQueueTimeSlice gets a specific unbonding queue timeslice. A timeslice
// is a slice of DVPairs corresponding to unbonding delegations that expire at a
// certain time.
func (k Keeper) GetUBDQueueTimeSlice(ctx context.Context, timestamp time.Time) (dvPairs []types.DVPair, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetUBDQueueTimeSlice")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)

	bz, err := store.Get(types.GetUnbondingDelegationTimeKey(timestamp))
	if bz == nil || err != nil {
		return []types.DVPair{}, err
	}

	pairs := types.DVPairs{}
	err = k.cdc.Unmarshal(bz, &pairs)

	return pairs.Pairs, err
}

// SetUBDQueueTimeSlice sets a specific unbonding queue timeslice.
func (k Keeper) SetUBDQueueTimeSlice(ctx context.Context, timestamp time.Time, keys []types.DVPair) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "SetUBDQueueTimeSlice")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	bz, err := k.cdc.Marshal(&types.DVPairs{Pairs: keys})
	if err != nil {
		return err
	}
	return store.Set(types.GetUnbondingDelegationTimeKey(timestamp), bz)
}

// InsertUBDQueue inserts an unbonding delegation to the appropriate timeslice
// in the unbonding queue.
func (k Keeper) InsertUBDQueue(ctx context.Context, ubd types.UnbondingDelegation, completionTime time.Time) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "InsertUBDQueue")(&err)

	dvPair := types.DVPair{DelegatorAddress: ubd.DelegatorAddress, ValidatorAddress: ubd.ValidatorAddress}

	timeSlice, err := k.GetUBDQueueTimeSlice(sdkCtx, completionTime)
	if err != nil {
		return err
	}

	if len(timeSlice) == 0 {
		if err = k.SetUBDQueueTimeSlice(sdkCtx, completionTime, []types.DVPair{dvPair}); err != nil {
			return err
		}
		return nil
	}

	timeSlice = append(timeSlice, dvPair)
	return k.SetUBDQueueTimeSlice(sdkCtx, completionTime, timeSlice)
}

// UBDQueueIterator returns all the unbonding queue timeslices from time 0 until endTime.
func (k Keeper) UBDQueueIterator(ctx context.Context, endTime time.Time) (meterResult corestore.Iterator, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "UBDQueueIterator")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	return store.Iterator(types.UnbondingQueueKey,
		storetypes.InclusiveEndBytes(types.GetUnbondingDelegationTimeKey(endTime)))
}

// DequeueAllMatureUBDQueue returns a concatenated list of all the timeslices inclusively previous to
// currTime, and deletes the timeslices from the queue.
func (k Keeper) DequeueAllMatureUBDQueue(ctx context.Context, currTime time.Time) (matureUnbonds []types.DVPair, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "DequeueAllMatureUBDQueue")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)

	// gets an iterator for all timeslices from time 0 until the current Blockheader time
	unbondingTimesliceIterator, err := k.UBDQueueIterator(sdkCtx, currTime)
	if err != nil {
		return matureUnbonds, err
	}
	defer unbondingTimesliceIterator.Close()

	for ; unbondingTimesliceIterator.Valid(); unbondingTimesliceIterator.Next() {
		timeslice := types.DVPairs{}
		value := unbondingTimesliceIterator.Value()
		if err = k.cdc.Unmarshal(value, &timeslice); err != nil {
			return matureUnbonds, err
		}

		matureUnbonds = append(matureUnbonds, timeslice.Pairs...)

		if err = store.Delete(unbondingTimesliceIterator.Key()); err != nil {
			return matureUnbonds, err
		}

	}

	return matureUnbonds, nil
}

// GetRedelegations returns a given amount of all the delegator redelegations.
func (k Keeper) GetRedelegations(ctx context.Context, delegator sdk.AccAddress, maxRetrieve uint16) (redelegations []types.Redelegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetRedelegations")(&err)

	redelegations = make([]types.Redelegation, maxRetrieve)

	store := k.storeService.OpenKVStore(sdkCtx)
	delegatorPrefixKey := types.GetREDsKey(delegator)
	iterator, err := store.Iterator(delegatorPrefixKey, storetypes.PrefixEndBytes(delegatorPrefixKey))
	if err != nil {
		return nil, err
	}

	i := 0
	for ; iterator.Valid() && i < int(maxRetrieve); iterator.Next() {
		redelegation, err := types.UnmarshalRED(k.cdc, iterator.Value())
		if err != nil {
			return nil, err
		}
		redelegations[i] = redelegation
		i++
	}

	return redelegations[:i], nil // trim if the array length < maxRetrieve
}

// GetRedelegation returns a redelegation.
func (k Keeper) GetRedelegation(ctx context.Context, delAddr sdk.AccAddress, valSrcAddr, valDstAddr sdk.ValAddress) (red types.Redelegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetRedelegation")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	key := types.GetREDKey(delAddr, valSrcAddr, valDstAddr)

	value, err := store.Get(key)
	if err != nil {
		return red, err
	}

	if value == nil {
		return red, types.ErrNoRedelegation
	}

	return types.UnmarshalRED(k.cdc, value)
}

// GetRedelegationsFromSrcValidator returns all redelegations from a particular
// validator.
func (k Keeper) GetRedelegationsFromSrcValidator(ctx context.Context, valAddr sdk.ValAddress) (reds []types.Redelegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetRedelegationsFromSrcValidator")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	prefix := types.GetREDsFromValSrcIndexKey(valAddr)
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		key := types.GetREDKeyFromValSrcIndexKey(iterator.Key())
		value, err := store.Get(key)
		if err != nil {
			return nil, err
		}
		red, err := types.UnmarshalRED(k.cdc, value)
		if err != nil {
			return nil, err
		}
		reds = append(reds, red)
	}

	return reds, nil
}

// HasReceivingRedelegation checks if validator is receiving a redelegation.
func (k Keeper) HasReceivingRedelegation(ctx context.Context, delAddr sdk.AccAddress, valDstAddr sdk.ValAddress) (meterResult bool, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "HasReceivingRedelegation")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	prefix := types.GetREDsByDelToValDstIndexKey(delAddr, valDstAddr)
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return false, err
	}
	defer iterator.Close()
	return iterator.Valid(), nil
}

// HasMaxRedelegationEntries checks if the redelegation entries reached maximum limit.
func (k Keeper) HasMaxRedelegationEntries(ctx context.Context, delegatorAddr sdk.AccAddress, validatorSrcAddr, validatorDstAddr sdk.ValAddress) (meterResult bool, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "HasMaxRedelegationEntries")(&err)

	red, err := k.GetRedelegation(sdkCtx, delegatorAddr, validatorSrcAddr, validatorDstAddr)
	if err != nil {
		if err == types.ErrNoRedelegation {
			return false, nil
		}

		return false, err
	}
	maxEntries, err := k.MaxEntries(sdkCtx)
	if err != nil {
		return false, err
	}

	return len(red.Entries) >= int(maxEntries), nil
}

// SetRedelegation sets a redelegation and associated index.
func (k Keeper) SetRedelegation(ctx context.Context, red types.Redelegation) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "SetRedelegation")(&err)

	delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(red.DelegatorAddress)
	if err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(sdkCtx)
	bz := types.MustMarshalRED(k.cdc, red)
	valSrcAddr, err := k.validatorAddressCodec.StringToBytes(red.ValidatorSrcAddress)
	if err != nil {
		return err
	}
	valDestAddr, err := k.validatorAddressCodec.StringToBytes(red.ValidatorDstAddress)
	if err != nil {
		return err
	}
	key := types.GetREDKey(delegatorAddress, valSrcAddr, valDestAddr)
	if err = store.Set(key, bz); err != nil {
		return err
	}

	if err = store.Set(types.GetREDByValSrcIndexKey(delegatorAddress, valSrcAddr, valDestAddr), []byte{}); err != nil {
		return err
	}

	return store.Set(types.GetREDByValDstIndexKey(delegatorAddress, valSrcAddr, valDestAddr), []byte{})
}

// SetRedelegationEntry adds an entry to the unbonding delegation at the given
// addresses. It creates the unbonding delegation if it does not exist.
func (k Keeper) SetRedelegationEntry(ctx context.Context,
	delegatorAddr sdk.AccAddress, validatorSrcAddr,
	validatorDstAddr sdk.ValAddress, creationHeight int64,
	minTime time.Time, balance math.Int,
	sharesSrc, sharesDst math.LegacyDec,
) (meterResult types.Redelegation, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "SetRedelegationEntry")(&err)

	id, err := k.IncrementUnbondingID(sdkCtx)
	if err != nil {
		return types.Redelegation{}, err
	}

	red, err := k.GetRedelegation(sdkCtx, delegatorAddr, validatorSrcAddr, validatorDstAddr)
	if err == nil {
		red.AddEntry(creationHeight, minTime, balance, sharesDst, id)
	} else if errors.Is(err, types.ErrNoRedelegation) {
		red = types.NewRedelegation(delegatorAddr, validatorSrcAddr,
			validatorDstAddr, creationHeight, minTime, balance, sharesDst, id, k.validatorAddressCodec, k.authKeeper.AddressCodec())
	} else {
		return types.Redelegation{}, err
	}

	if err = k.SetRedelegation(sdkCtx, red); err != nil {
		return types.Redelegation{}, err
	}

	// Add to the UBDByEntry index to look up the UBD by the UBDE ID
	if err = k.SetRedelegationByUnbondingID(sdkCtx, red, id); err != nil {
		return types.Redelegation{}, err
	}

	if err = k.Hooks().AfterUnbondingInitiated(sdkCtx, id); err != nil {
		k.Logger(sdkCtx).Error("failed to call after unbonding initiated hook", "error", err)
		// TODO (Facu): Should we return here? We are ignoring this error
	}

	return red, nil
}

// IterateRedelegations iterates through all redelegations.
func (k Keeper) IterateRedelegations(ctx context.Context, fn func(index int64, red types.Redelegation) (stop bool)) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "IterateRedelegations")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	iterator, err := store.Iterator(types.RedelegationKey, storetypes.PrefixEndBytes(types.RedelegationKey))
	if err != nil {
		return err
	}
	defer iterator.Close()

	for i := int64(0); iterator.Valid(); iterator.Next() {
		red, err := types.UnmarshalRED(k.cdc, iterator.Value())
		if err != nil {
			return err
		}
		if stop := fn(i, red); stop {
			break
		}
		i++
	}

	return nil
}

// RemoveRedelegation removes a redelegation object and associated index.
func (k Keeper) RemoveRedelegation(ctx context.Context, red types.Redelegation) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "RemoveRedelegation")(&err)

	delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(red.DelegatorAddress)
	if err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(sdkCtx)
	valSrcAddr, err := k.validatorAddressCodec.StringToBytes(red.ValidatorSrcAddress)
	if err != nil {
		return err
	}
	valDestAddr, err := k.validatorAddressCodec.StringToBytes(red.ValidatorDstAddress)
	if err != nil {
		return err
	}
	redKey := types.GetREDKey(delegatorAddress, valSrcAddr, valDestAddr)
	if err = store.Delete(redKey); err != nil {
		return err
	}

	if err = store.Delete(types.GetREDByValSrcIndexKey(delegatorAddress, valSrcAddr, valDestAddr)); err != nil {
		return err
	}

	return store.Delete(types.GetREDByValDstIndexKey(delegatorAddress, valSrcAddr, valDestAddr))
}

// redelegation queue timeslice operations

// GetRedelegationQueueTimeSlice gets a specific redelegation queue timeslice. A
// timeslice is a slice of DVVTriplets corresponding to redelegations that
// expire at a certain time.
func (k Keeper) GetRedelegationQueueTimeSlice(ctx context.Context, timestamp time.Time) (dvvTriplets []types.DVVTriplet, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetRedelegationQueueTimeSlice")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	bz, err := store.Get(types.GetRedelegationTimeKey(timestamp))
	if err != nil {
		return nil, err
	}

	if bz == nil {
		return []types.DVVTriplet{}, nil
	}

	triplets := types.DVVTriplets{}
	err = k.cdc.Unmarshal(bz, &triplets)
	if err != nil {
		return nil, err
	}

	return triplets.Triplets, nil
}

// SetRedelegationQueueTimeSlice sets a specific redelegation queue timeslice.
func (k Keeper) SetRedelegationQueueTimeSlice(ctx context.Context, timestamp time.Time, keys []types.DVVTriplet) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "SetRedelegationQueueTimeSlice")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	bz, err := k.cdc.Marshal(&types.DVVTriplets{Triplets: keys})
	if err != nil {
		return err
	}
	return store.Set(types.GetRedelegationTimeKey(timestamp), bz)
}

// InsertRedelegationQueue insert an redelegation delegation to the appropriate
// timeslice in the redelegation queue.
func (k Keeper) InsertRedelegationQueue(ctx context.Context, red types.Redelegation, completionTime time.Time) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "InsertRedelegationQueue")(&err)

	timeSlice, err := k.GetRedelegationQueueTimeSlice(sdkCtx, completionTime)
	if err != nil {
		return err
	}
	dvvTriplet := types.DVVTriplet{
		DelegatorAddress:    red.DelegatorAddress,
		ValidatorSrcAddress: red.ValidatorSrcAddress,
		ValidatorDstAddress: red.ValidatorDstAddress,
	}

	if len(timeSlice) == 0 {
		return k.SetRedelegationQueueTimeSlice(sdkCtx, completionTime, []types.DVVTriplet{dvvTriplet})
	}

	timeSlice = append(timeSlice, dvvTriplet)
	return k.SetRedelegationQueueTimeSlice(sdkCtx, completionTime, timeSlice)
}

// RedelegationQueueIterator returns all the redelegation queue timeslices from
// time 0 until endTime.
func (k Keeper) RedelegationQueueIterator(ctx context.Context, endTime time.Time) (meterResult storetypes.Iterator, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "RedelegationQueueIterator")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	return store.Iterator(types.RedelegationQueueKey, storetypes.InclusiveEndBytes(types.GetRedelegationTimeKey(endTime)))
}

// DequeueAllMatureRedelegationQueue returns a concatenated list of all the
// timeslices inclusively previous to currTime, and deletes the timeslices from
// the queue.
func (k Keeper) DequeueAllMatureRedelegationQueue(ctx context.Context, currTime time.Time) (matureRedelegations []types.DVVTriplet, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "DequeueAllMatureRedelegationQueue")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)

	// gets an iterator for all timeslices from time 0 until the current Blockheader time
	redelegationTimesliceIterator, err := k.RedelegationQueueIterator(sdkCtx, sdkCtx.HeaderInfo().Time)
	if err != nil {
		return nil, err
	}
	defer redelegationTimesliceIterator.Close()

	for ; redelegationTimesliceIterator.Valid(); redelegationTimesliceIterator.Next() {
		timeslice := types.DVVTriplets{}
		value := redelegationTimesliceIterator.Value()
		if err = k.cdc.Unmarshal(value, &timeslice); err != nil {
			return nil, err
		}

		matureRedelegations = append(matureRedelegations, timeslice.Triplets...)

		if err = store.Delete(redelegationTimesliceIterator.Key()); err != nil {
			return nil, err
		}
	}

	return matureRedelegations, nil
}

// Delegate performs a delegation, set/update everything necessary within the store.
// tokenSrc indicates the bond status of the incoming funds.
func (k Keeper) Delegate(
	ctx context.Context, delAddr sdk.AccAddress, bondAmt math.Int, tokenSrc types.BondStatus,
	validator types.Validator, subtractAccount bool,
) (newShares math.LegacyDec, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "Delegate")(&err)

	// In some situations, the exchange rate becomes invalid, e.g. if
	// Validator loses all tokens due to slashing. In this case,
	// make all future delegations invalid.
	if validator.InvalidExRate() {
		return math.LegacyZeroDec(), types.ErrDelegatorShareExRateInvalid
	}

	valbz, err := k.ValidatorAddressCodec().StringToBytes(validator.GetOperator())
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	// Get or create the delegation object and call the appropriate hook if present
	delegation, err := k.GetDelegation(sdkCtx, delAddr, valbz)
	if err == nil {
		// found
		err = k.Hooks().BeforeDelegationSharesModified(sdkCtx, delAddr, valbz)
	} else if errors.Is(err, types.ErrNoDelegation) {
		// not found
		delAddrStr, err1 := k.authKeeper.AddressCodec().BytesToString(delAddr)
		if err1 != nil {
			return math.LegacyDec{}, err1
		}

		delegation = types.NewDelegation(delAddrStr, validator.GetOperator(), math.LegacyZeroDec())
		err = k.Hooks().BeforeDelegationCreated(sdkCtx, delAddr, valbz)
	} else {
		return math.LegacyZeroDec(), err
	}

	if err != nil {
		return math.LegacyZeroDec(), err
	}

	// if subtractAccount is true then we are
	// performing a delegation and not a redelegation, thus the source tokens are
	// all non bonded
	if subtractAccount {
		if tokenSrc == types.Bonded {
			panic("delegation token source cannot be bonded")
		}

		var sendName string

		switch {
		case validator.IsBonded():
			sendName = types.BondedPoolName
		case validator.IsUnbonding(), validator.IsUnbonded():
			sendName = types.NotBondedPoolName
		default:
			panic("invalid validator status")
		}

		bondDenom, err := k.BondDenom(sdkCtx)
		if err != nil {
			return math.LegacyDec{}, err
		}

		coins := sdk.NewCoins(sdk.NewCoin(bondDenom, bondAmt))
		if err = k.bankKeeper.DelegateCoinsFromAccountToModule(sdkCtx, delAddr, sendName, coins); err != nil {
			return math.LegacyDec{}, err
		}
	} else {
		// potentially transfer tokens between pools, if
		switch {
		case tokenSrc == types.Bonded && validator.IsBonded():
			// do nothing
		case (tokenSrc == types.Unbonded || tokenSrc == types.Unbonding) && !validator.IsBonded():
			// do nothing
		case (tokenSrc == types.Unbonded || tokenSrc == types.Unbonding) && validator.IsBonded():
			// transfer pools
			err = k.notBondedTokensToBonded(sdkCtx, bondAmt)
			if err != nil {
				return math.LegacyDec{}, err
			}
		case tokenSrc == types.Bonded && !validator.IsBonded():
			// transfer pools
			err = k.bondedTokensToNotBonded(sdkCtx, bondAmt)
			if err != nil {
				return math.LegacyDec{}, err
			}
		default:
			panic("unknown token source bond status")
		}
	}

	_, newShares, err = k.AddValidatorTokensAndShares(sdkCtx, validator, bondAmt)
	if err != nil {
		return newShares, err
	}

	// Update delegation
	delegation.Shares = delegation.Shares.Add(newShares)
	if err = k.SetDelegation(sdkCtx, delegation); err != nil {
		return newShares, err
	}

	// Call the after-modification hook
	if err = k.Hooks().AfterDelegationModified(sdkCtx, delAddr, valbz); err != nil {
		return newShares, err
	}

	return newShares, nil
}

// Unbond unbonds a particular delegation and perform associated store operations.
func (k Keeper) Unbond(
	ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, shares math.LegacyDec,
) (amount math.Int, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "Unbond")(&err)

	// check if a delegation object exists in the store
	delegation, err := k.GetDelegation(sdkCtx, delAddr, valAddr)
	if errors.Is(err, types.ErrNoDelegation) {
		return amount, types.ErrNoDelegatorForAddress
	} else if err != nil {
		return amount, err
	}

	// call the before-delegation-modified hook
	if err = k.Hooks().BeforeDelegationSharesModified(sdkCtx, delAddr, valAddr); err != nil {
		return amount, err
	}

	// ensure that we have enough shares to remove
	if delegation.Shares.LT(shares) {
		return amount, errorsmod.Wrap(types.ErrNotEnoughDelegationShares, delegation.Shares.String())
	}

	// get validator
	validator, err := k.GetValidator(sdkCtx, valAddr)
	if err != nil {
		return amount, err
	}

	// subtract shares from delegation
	delegation.Shares = delegation.Shares.Sub(shares)

	delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(delegation.DelegatorAddress)
	if err != nil {
		return amount, err
	}

	valbz, err := k.ValidatorAddressCodec().StringToBytes(validator.GetOperator())
	if err != nil {
		return amount, err
	}

	isValidatorOperator := bytes.Equal(delegatorAddress, valbz)

	// If the delegation is the operator of the validator and undelegating will decrease the validator's
	// self-delegation below their minimum, we jail the validator.
	if isValidatorOperator && !validator.Jailed &&
		validator.TokensFromShares(delegation.Shares).TruncateInt().LT(validator.MinSelfDelegation) {
		err = k.jailValidator(sdkCtx, validator)
		if err != nil {
			return amount, err
		}
		validator = k.mustGetValidator(sdkCtx, valbz)
	}

	if delegation.Shares.IsZero() {
		err = k.RemoveDelegation(sdkCtx, delegation)
	} else {
		if err = k.SetDelegation(sdkCtx, delegation); err != nil {
			return amount, err
		}

		valAddr, err1 := k.validatorAddressCodec.StringToBytes(delegation.GetValidatorAddr())
		if err1 != nil {
			return amount, err1
		}

		// call the after delegation modification hook
		err = k.Hooks().AfterDelegationModified(sdkCtx, delegatorAddress, valAddr)
	}

	if err != nil {
		return amount, err
	}

	// remove the shares and coins from the validator
	// NOTE that the amount is later (in keeper.Delegation) moved between staking module pools
	validator, amount, err = k.RemoveValidatorTokensAndShares(sdkCtx, validator, shares)
	if err != nil {
		return amount, err
	}

	if validator.DelegatorShares.IsZero() && validator.IsUnbonded() {
		// if not unbonded, we must instead remove validator in EndBlocker once it finishes its unbonding period
		if err = k.RemoveValidator(sdkCtx, valbz); err != nil {
			return amount, err
		}
	}

	return amount, nil
}

// getBeginInfo returns the completion time and height of a redelegation, along
// with a boolean signaling if the redelegation is complete based on the source
// validator.
func (k Keeper) getBeginInfo(
	ctx context.Context, valSrcAddr sdk.ValAddress,
) (completionTime time.Time, height int64, completeNow bool, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "getBeginInfo")(&err)

	validator, err := k.GetValidator(sdkCtx, valSrcAddr)
	if err != nil && errors.Is(err, types.ErrNoValidatorFound) {
		return
	}
	unbondingTime, err := k.UnbondingTime(sdkCtx)
	if err != nil {
		return
	}

	// TODO: When would the validator not be found?
	switch {
	case errors.Is(err, types.ErrNoValidatorFound) || validator.IsBonded():
		// the longest wait - just unbonding period from now
		completionTime = sdkCtx.BlockHeader().Time.Add(unbondingTime)
		height = sdkCtx.BlockHeight()

		return completionTime, height, false, nil

	case validator.IsUnbonded():
		return completionTime, height, true, nil

	case validator.IsUnbonding():
		return validator.UnbondingTime, validator.UnbondingHeight, false, nil

	default:
		panic(fmt.Sprintf("unknown validator status: %s", validator.Status))
	}
}

// Undelegate unbonds an amount of delegator shares from a given validator. It
// will verify that the unbonding entries between the delegator and validator
// are not exceeded and unbond the staked tokens (based on shares) by creating
// an unbonding object and inserting it into the unbonding queue which will be
// processed during the staking EndBlocker.
func (k Keeper) Undelegate(
	ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, sharesAmount math.LegacyDec,
) (meterResult time.Time, meterResult1 math.Int, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "Undelegate")(&err)

	validator, err := k.GetValidator(sdkCtx, valAddr)
	if err != nil {
		return time.Time{}, math.Int{}, err
	}

	hasMaxEntries, err := k.HasMaxUnbondingDelegationEntries(sdkCtx, delAddr, valAddr)
	if err != nil {
		return time.Time{}, math.Int{}, err
	}

	if hasMaxEntries {
		return time.Time{}, math.Int{}, types.ErrMaxUnbondingDelegationEntries
	}

	returnAmount, err := k.Unbond(sdkCtx, delAddr, valAddr, sharesAmount)
	if err != nil {
		return time.Time{}, math.Int{}, err
	}

	// transfer the validator tokens to the not bonded pool
	if validator.IsBonded() {
		err = k.bondedTokensToNotBonded(sdkCtx, returnAmount)
		if err != nil {
			return time.Time{}, math.Int{}, err
		}
	}

	unbondingTime, err := k.UnbondingTime(sdkCtx)
	if err != nil {
		return time.Time{}, math.Int{}, err
	}
	completionTime := sdkCtx.BlockHeader().Time.Add(unbondingTime)
	ubd, err := k.SetUnbondingDelegationEntry(sdkCtx, delAddr, valAddr, sdkCtx.BlockHeight(), completionTime, returnAmount)
	if err != nil {
		return time.Time{}, math.Int{}, err
	}

	err = k.InsertUBDQueue(sdkCtx, ubd, completionTime)
	if err != nil {
		return time.Time{}, math.Int{}, err
	}

	return completionTime, returnAmount, nil
}

// CompleteUnbonding completes the unbonding of all mature entries in the
// retrieved unbonding delegation object and returns the total unbonding balance
// or an error upon failure.
func (k Keeper) CompleteUnbonding(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (meterResult sdk.Coins, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "CompleteUnbonding")(&err)

	ubd, err := k.GetUnbondingDelegation(sdkCtx, delAddr, valAddr)
	if err != nil {
		return nil, err
	}

	bondDenom, err := k.BondDenom(sdkCtx)
	if err != nil {
		return nil, err
	}

	balances := sdk.NewCoins()
	ctxTime := sdkCtx.BlockHeader().Time

	delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(ubd.DelegatorAddress)
	if err != nil {
		return nil, err
	}

	// loop through all the entries and complete unbonding mature entries
	for i := 0; i < len(ubd.Entries); i++ {
		entry := ubd.Entries[i]
		if entry.IsMature(ctxTime) && !entry.OnHold() {
			ubd.RemoveEntry(int64(i))
			i--
			if err = k.DeleteUnbondingIndex(sdkCtx, entry.UnbondingId); err != nil {
				return nil, err
			}

			// track undelegation only when remaining or truncated shares are non-zero
			if !entry.Balance.IsZero() {
				amt := sdk.NewCoin(bondDenom, entry.Balance)
				if err = k.bankKeeper.UndelegateCoinsFromModuleToAccount(
					sdkCtx, types.NotBondedPoolName, delegatorAddress, sdk.NewCoins(amt),
				); err != nil {
					return nil, err
				}

				balances = balances.Add(amt)
			}
		}
	}

	// set the unbonding delegation or remove it if there are no more entries
	if len(ubd.Entries) == 0 {
		err = k.RemoveUnbondingDelegation(sdkCtx, ubd)
	} else {
		err = k.SetUnbondingDelegation(sdkCtx, ubd)
	}

	if err != nil {
		return nil, err
	}

	return balances, nil
}

// BeginRedelegation begins unbonding / redelegation and creates a redelegation
// record.
func (k Keeper) BeginRedelegation(
	ctx context.Context, delAddr sdk.AccAddress, valSrcAddr, valDstAddr sdk.ValAddress, sharesAmount math.LegacyDec,
) (completionTime time.Time, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "BeginRedelegation")(&err)

	if bytes.Equal(valSrcAddr, valDstAddr) {
		return time.Time{}, types.ErrSelfRedelegation
	}

	dstValidator, err := k.GetValidator(sdkCtx, valDstAddr)
	if errors.Is(err, types.ErrNoValidatorFound) {
		return time.Time{}, types.ErrBadRedelegationDst
	} else if err != nil {
		return time.Time{}, err
	}

	srcValidator, err := k.GetValidator(sdkCtx, valSrcAddr)
	if errors.Is(err, types.ErrNoValidatorFound) {
		return time.Time{}, types.ErrBadRedelegationSrc
	} else if err != nil {
		return time.Time{}, err
	}

	// check if this is a transitive redelegation
	hasRecRedel, err := k.HasReceivingRedelegation(sdkCtx, delAddr, valSrcAddr)
	if err != nil {
		return time.Time{}, err
	}

	if hasRecRedel {
		return time.Time{}, types.ErrTransitiveRedelegation
	}

	hasMaxRedels, err := k.HasMaxRedelegationEntries(sdkCtx, delAddr, valSrcAddr, valDstAddr)
	if err != nil {
		return time.Time{}, err
	}

	if hasMaxRedels {
		return time.Time{}, types.ErrMaxRedelegationEntries
	}

	returnAmount, err := k.Unbond(sdkCtx, delAddr, valSrcAddr, sharesAmount)
	if err != nil {
		return time.Time{}, err
	}

	if returnAmount.IsZero() {
		return time.Time{}, types.ErrTinyRedelegationAmount
	}

	sharesCreated, err := k.Delegate(sdkCtx, delAddr, returnAmount, srcValidator.GetStatus(), dstValidator, false)
	if err != nil {
		return time.Time{}, err
	}

	// create the unbonding delegation
	completionTime, height, completeNow, err := k.getBeginInfo(sdkCtx, valSrcAddr)
	if err != nil {
		return time.Time{}, err
	}

	if completeNow { // no need to create the redelegation object
		return completionTime, nil
	}

	red, err := k.SetRedelegationEntry(
		sdkCtx, delAddr, valSrcAddr, valDstAddr,
		height, completionTime, returnAmount, sharesAmount, sharesCreated,
	)
	if err != nil {
		return time.Time{}, err
	}

	err = k.InsertRedelegationQueue(sdkCtx, red, completionTime)
	if err != nil {
		return time.Time{}, err
	}

	return completionTime, nil
}

// CompleteRedelegation completes the redelegations of all mature entries in the
// retrieved redelegation object and returns the total redelegation (initial)
// balance or an error upon failure.
func (k Keeper) CompleteRedelegation(
	ctx context.Context, delAddr sdk.AccAddress, valSrcAddr, valDstAddr sdk.ValAddress,
) (meterResult sdk.Coins, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "CompleteRedelegation")(&err)

	red, err := k.GetRedelegation(sdkCtx, delAddr, valSrcAddr, valDstAddr)
	if err != nil {
		return nil, err
	}

	bondDenom, err := k.BondDenom(sdkCtx)
	if err != nil {
		return nil, err
	}

	balances := sdk.NewCoins()
	ctxTime := sdkCtx.BlockHeader().Time

	// loop through all the entries and complete mature redelegation entries
	for i := 0; i < len(red.Entries); i++ {
		entry := red.Entries[i]
		if entry.IsMature(ctxTime) && !entry.OnHold() {
			red.RemoveEntry(int64(i))
			i--
			if err = k.DeleteUnbondingIndex(sdkCtx, entry.UnbondingId); err != nil {
				return nil, err
			}

			if !entry.InitialBalance.IsZero() {
				balances = balances.Add(sdk.NewCoin(bondDenom, entry.InitialBalance))
			}
		}
	}

	// set the redelegation or remove it if there are no more entries
	if len(red.Entries) == 0 {
		err = k.RemoveRedelegation(sdkCtx, red)
	} else {
		err = k.SetRedelegation(sdkCtx, red)
	}

	if err != nil {
		return nil, err
	}

	return balances, nil
}

// ValidateUnbondAmount validates that a given unbond or redelegation amount is
// valid based on upon the converted shares. If the amount is valid, the total
// amount of respective shares is returned, otherwise an error is returned.
func (k Keeper) ValidateUnbondAmount(
	ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, amt math.Int,
) (shares math.LegacyDec, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "ValidateUnbondAmount")(&err)

	validator, err := k.GetValidator(sdkCtx, valAddr)
	if err != nil {
		return shares, err
	}

	del, err := k.GetDelegation(sdkCtx, delAddr, valAddr)
	if err != nil {
		return shares, err
	}

	shares, err = validator.SharesFromTokens(amt)
	if err != nil {
		return shares, err
	}

	sharesTruncated, err := validator.SharesFromTokensTruncated(amt)
	if err != nil {
		return shares, err
	}

	delShares := del.GetShares()
	if sharesTruncated.GT(delShares) {
		return shares, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "invalid shares amount")
	}

	// Cap the shares at the delegation's shares. Shares being greater could occur
	// due to rounding, however we don't want to truncate the shares or take the
	// minimum because we want to allow for the full withdraw of shares from a
	// delegation.
	if shares.GT(delShares) {
		shares = delShares
	}

	return shares, nil
}
