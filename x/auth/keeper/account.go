package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewAccountWithAddress implements AccountKeeperI.
func (ak AccountKeeper) NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer ak.Meter(sdkCtx).FuncTiming(&sdkCtx, "NewAccountWithAddress")()

	acc := ak.proto()
	err := acc.SetAddress(addr)
	if err != nil {
		panic(err)
	}

	return ak.NewAccount(sdkCtx, acc)
}

// NewAccount sets the next account number to a given account interface
func (ak AccountKeeper) NewAccount(ctx context.Context, acc sdk.AccountI) sdk.AccountI {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer ak.Meter(sdkCtx).FuncTiming(&sdkCtx, "NewAccount")()

	if err := acc.SetAccountNumber(ak.NextAccountNumber(sdkCtx)); err != nil {
		panic(err)
	}

	return acc
}

// HasAccount implements AccountKeeperI.
func (ak AccountKeeper) HasAccount(ctx context.Context, addr sdk.AccAddress) bool {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer ak.Meter(sdkCtx).FuncTiming(&sdkCtx, "HasAccount")()

	has, _ := ak.Accounts.Has(sdkCtx, addr)
	return has
}

// GetAccount implements AccountKeeperI.
func (ak AccountKeeper) GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer ak.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetAccount")()

	acc, err := ak.Accounts.Get(sdkCtx, addr)
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		panic(err)
	}
	return acc
}

// GetAllAccounts returns all accounts in the accountKeeper.
func (ak AccountKeeper) GetAllAccounts(ctx context.Context) (accounts []sdk.AccountI) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer ak.Meter(sdkCtx).FuncTiming(&sdkCtx, "GetAllAccounts")()

	ak.IterateAccounts(sdkCtx, func(acc sdk.AccountI) (stop bool) {
		accounts = append(accounts, acc)
		return false
	})

	return accounts
}

// SetAccount implements AccountKeeperI.
func (ak AccountKeeper) SetAccount(ctx context.Context, acc sdk.AccountI) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer ak.Meter(sdkCtx).FuncTiming(&sdkCtx, "SetAccount")()

	err := ak.Accounts.Set(sdkCtx, acc.GetAddress(), acc)
	if err != nil {
		panic(err)
	}
}

// RemoveAccount removes an account for the account mapper store.
// NOTE: this will cause supply invariant violation if called
func (ak AccountKeeper) RemoveAccount(ctx context.Context, acc sdk.AccountI) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer ak.Meter(sdkCtx).FuncTiming(&sdkCtx, "RemoveAccount")()

	err := ak.Accounts.Remove(sdkCtx, acc.GetAddress())
	if err != nil {
		panic(err)
	}
}

// IterateAccounts iterates over all the stored accounts and performs a callback function.
// Stops iteration when callback returns true.
func (ak AccountKeeper) IterateAccounts(ctx context.Context, cb func(account sdk.AccountI) (stop bool)) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer ak.Meter(sdkCtx).FuncTiming(&sdkCtx, "IterateAccounts")()

	err := ak.Accounts.Walk(sdkCtx, nil, func(_ sdk.AccAddress, value sdk.AccountI) (bool, error) {
		return cb(value), nil
	})
	if err != nil {
		panic(err)
	}
}
