package keeper_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	cmtproto "github.com/cometbft/cometbft/api/cometbft/types/v1"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var errUnbondingWrite = errors.New("unbonding store write failed")

type failingUnbondingStoreService struct {
	corestore.KVStoreService
	failDelete []byte
}

func (s *failingUnbondingStoreService) OpenKVStore(ctx context.Context) corestore.KVStore {
	return failingUnbondingStore{s.KVStoreService.OpenKVStore(ctx), s.failDelete}
}

type failingUnbondingStore struct {
	corestore.KVStore
	failDelete []byte
}

func (s failingUnbondingStore) Delete(key []byte) error {
	if bytes.Equal(key, s.failDelete) {
		return errUnbondingWrite
	}
	return s.KVStore.Delete(key)
}

type unbondingFixture struct {
	ctx     sdk.Context
	auth    authkeeper.AccountKeeper
	bank    bankkeeper.BaseKeeper
	staking *stakingkeeper.Keeper
	store   *failingUnbondingStoreService
}

func newUnbondingFixture(t *testing.T) unbondingFixture {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	keys := storetypes.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, db)
	}
	tkey := storetypes.NewTransientStoreKey(banktypes.TStoreKey)
	okey := storetypes.NewObjectStoreKey(banktypes.ObjectStoreKey)
	cms.MountStoreWithDB(tkey, storetypes.StoreTypeTransient, nil)
	cms.MountStoreWithDB(okey, storetypes.StoreTypeObject, nil)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx := sdk.NewContext(cms, cmtproto.Header{Height: 10, Time: time.Unix(1000, 0).UTC()}, false, log.NewNopLogger())
	enc := moduletestutil.MakeTestEncodingConfig()
	authtypes.RegisterInterfaces(enc.InterfaceRegistry)
	banktypes.RegisterInterfaces(enc.InterfaceRegistry)
	stakingtypes.RegisterInterfaces(enc.InterfaceRegistry)
	permissions := map[string][]string{
		"mint":                         {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	}
	authority := authtypes.NewModuleAddress("gov").String()
	ak := authkeeper.NewAccountKeeper(enc.Codec, runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount, permissions, address.NewBech32Codec("cosmos"), "cosmos", authority)
	bk := bankkeeper.NewBaseKeeper(enc.Codec, runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		runtime.NewTransientStoreService(tkey), okey, ak, nil, authority, log.NewNopLogger())
	stakingStore := &failingUnbondingStoreService{KVStoreService: runtime.NewKVStoreService(keys[stakingtypes.StoreKey])}
	sk := stakingkeeper.NewKeeper(enc.Codec, stakingStore, ak, bk, authority,
		address.NewBech32Codec("cosmosvaloper"), address.NewBech32Codec("cosmosvalcons"))
	require.NoError(t, bk.SetParams(ctx, banktypes.DefaultParams()))
	require.NoError(t, sk.SetParams(ctx, stakingtypes.DefaultParams()))
	return unbondingFixture{ctx: ctx, auth: ak, bank: bk, staking: sk, store: stakingStore}
}

func TestCompleteUnbondingRollsBackEarlierPayout(t *testing.T) {
	t.Parallel()
	f := newUnbondingFixture(t)
	ctx := f.ctx
	dels, vals := createValAddrs(1)
	del, val := dels[0], vals[0]
	f.auth.SetAccount(ctx, f.auth.NewAccountWithAddress(ctx, del))
	_, err := f.staking.SetUnbondingDelegationEntry(ctx, del, val, 1, ctx.BlockTime().Add(-2*time.Hour), math.NewInt(10))
	require.NoError(t, err)
	_, err = f.staking.SetUnbondingDelegationEntry(ctx, del, val, 2, ctx.BlockTime().Add(-time.Hour), math.NewInt(20))
	require.NoError(t, err)
	original, err := f.staking.SetUnbondingDelegationEntry(ctx, del, val, 3, ctx.BlockTime().Add(time.Hour), math.NewInt(30))
	require.NoError(t, err)
	coins := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10))
	require.NoError(t, banktestutil.FundModuleAccount(ctx, f.bank, stakingtypes.NotBondedPoolName, coins))
	pool := f.auth.GetModuleAddress(stakingtypes.NotBondedPoolName)
	events := append(sdk.Events(nil), ctx.EventManager().Events()...)
	updates := f.bank.GetAllTransientAccountBalanceUpdates(ctx)

	paid, err := f.staking.CompleteUnbonding(ctx, del, val)
	require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
	require.Nil(t, paid)
	require.Equal(t, coins, f.bank.GetAllBalances(ctx, pool))
	require.True(t, f.bank.GetAllBalances(ctx, del).Empty())
	stored, err := f.staking.GetUnbondingDelegation(ctx, del, val)
	require.NoError(t, err)
	require.Equal(t, original, stored)
	for _, entry := range original.Entries {
		indexed, err := f.staking.GetUnbondingDelegationByUnbondingID(ctx, entry.UnbondingId)
		require.NoError(t, err)
		require.Equal(t, original, indexed)
	}
	require.Equal(t, events, ctx.EventManager().Events())
	require.Equal(t, updates, f.bank.GetAllTransientAccountBalanceUpdates(ctx))

	require.NoError(t, banktestutil.FundModuleAccount(ctx, f.bank, stakingtypes.NotBondedPoolName,
		sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 20))))
	paid, err = f.staking.CompleteUnbonding(ctx, del, val)
	require.NoError(t, err)
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 30)), paid)
	stored, err = f.staking.GetUnbondingDelegation(ctx, del, val)
	require.NoError(t, err)
	require.Equal(t, original.Entries[2:], stored.Entries)
	for _, entry := range original.Entries[:2] {
		_, err := f.staking.GetUnbondingDelegationByUnbondingID(ctx, entry.UnbondingId)
		require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
	}
	require.Equal(t, paid, f.bank.GetAllBalances(ctx, del))
	require.True(t, f.bank.GetAllBalances(ctx, pool).Empty())

	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(2 * time.Hour))
	require.NoError(t, banktestutil.FundModuleAccount(ctx, f.bank, stakingtypes.NotBondedPoolName, paid))
	_, err = f.staking.CompleteUnbonding(ctx, del, val)
	require.NoError(t, err)
	_, err = f.staking.GetUnbondingDelegation(ctx, del, val)
	require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 60)), f.bank.GetAllBalances(ctx, del))
	message, broken := bankkeeper.TotalSupply(f.bank)(ctx)
	require.False(t, broken, message)
}

func TestHeldUnbondingRollsBackPayoutOnClaimWriteFailure(t *testing.T) {
	t.Parallel()
	f := newUnbondingFixture(t)
	ctx := f.ctx
	dels, vals := createValAddrs(1)
	del, val := dels[0], vals[0]
	f.auth.SetAccount(ctx, f.auth.NewAccountWithAddress(ctx, del))
	ubd, err := f.staking.SetUnbondingDelegationEntry(ctx, del, val, 1, ctx.BlockTime(), math.NewInt(10))
	require.NoError(t, err)
	id := ubd.Entries[0].UnbondingId
	require.NoError(t, f.staking.PutUnbondingOnHold(ctx, id))
	original, err := f.staking.GetUnbondingDelegation(ctx, del, val)
	require.NoError(t, err)
	coins := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10))
	require.NoError(t, banktestutil.FundModuleAccount(ctx, f.bank, stakingtypes.NotBondedPoolName, coins))
	pool := f.auth.GetModuleAddress(stakingtypes.NotBondedPoolName)
	events := append(sdk.Events(nil), ctx.EventManager().Events()...)
	updates := f.bank.GetAllTransientAccountBalanceUpdates(ctx)
	f.store.failDelete = stakingtypes.GetUBDKey(del, val)

	require.ErrorIs(t, f.staking.UnbondingCanComplete(ctx, id), errUnbondingWrite)
	require.Equal(t, coins, f.bank.GetAllBalances(ctx, pool))
	require.True(t, f.bank.GetAllBalances(ctx, del).Empty())
	stored, err := f.staking.GetUnbondingDelegationByUnbondingID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, original, stored)
	require.Equal(t, events, ctx.EventManager().Events())
	require.Equal(t, updates, f.bank.GetAllTransientAccountBalanceUpdates(ctx))

	f.store.failDelete = nil
	require.NoError(t, f.staking.UnbondingCanComplete(ctx, id))
	require.True(t, f.bank.GetAllBalances(ctx, pool).Empty())
	require.Equal(t, coins, f.bank.GetAllBalances(ctx, del))
	_, err = f.staking.GetUnbondingDelegationByUnbondingID(ctx, id)
	require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
}

func TestEndBlockDiscardsMissingAccountClaimsWithoutDuplicateBurns(t *testing.T) {
	t.Parallel()
	f := newUnbondingFixture(t)
	ctx := f.ctx
	dels, vals := createValAddrs(3)
	attacker, honest, val := dels[0], dels[1], vals[0]
	account := f.auth.NewAccountWithAddress(ctx, attacker)
	f.auth.SetAccount(ctx, account)
	f.auth.SetAccount(ctx, f.auth.NewAccountWithAddress(ctx, honest))
	for range 2 {
		ubd, err := f.staking.SetUnbondingDelegationEntry(ctx, attacker, val, ctx.BlockHeight(), ctx.BlockTime(), math.NewInt(10))
		require.NoError(t, err)
		require.NoError(t, f.staking.InsertUBDQueue(ctx, ubd, ctx.BlockTime()))
	}
	original, err := f.staking.GetUnbondingDelegation(ctx, attacker, val)
	require.NoError(t, err)
	require.Len(t, original.Entries, 1, "same-height entries must merge")
	require.NoError(t, f.staking.InsertUBDQueue(ctx, original, ctx.BlockTime().Add(-time.Second)))
	f.auth.RemoveAccount(ctx, account)
	ubd, err := f.staking.SetUnbondingDelegationEntry(ctx, honest, val, 1, ctx.BlockTime(), math.NewInt(30))
	require.NoError(t, err)
	require.NoError(t, f.staking.InsertUBDQueue(ctx, ubd, ctx.BlockTime()))
	stale := stakingtypes.UnbondingDelegation{DelegatorAddress: dels[2].String(), ValidatorAddress: val.String()}
	require.NoError(t, f.staking.InsertUBDQueue(ctx, stale, ctx.BlockTime()))
	require.NoError(t, banktestutil.FundModuleAccount(ctx, f.bank, stakingtypes.NotBondedPoolName,
		sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 50))))
	pool := f.auth.GetModuleAddress(stakingtypes.NotBondedPoolName)

	for block := range 3 {
		if block == 2 {
			// Account recreation must not restore a discarded claim.
			f.auth.SetAccount(ctx, f.auth.NewAccountWithAddress(ctx, attacker))
		}
		_, err := f.staking.EndBlocker(ctx)
		require.NoError(t, err)
		require.True(t, f.bank.GetAllBalances(ctx, pool).Empty())
		require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 30)), f.bank.GetAllBalances(ctx, honest))
		require.True(t, f.bank.GetAllBalances(ctx, attacker).Empty())
		_, err = f.staking.GetUnbondingDelegationByUnbondingID(ctx, original.Entries[0].UnbondingId)
		require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
		_, err = f.staking.GetUnbondingType(ctx, original.Entries[0].UnbondingId)
		require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingType)
		require.Equal(t, sdk.NewInt64Coin(sdk.DefaultBondDenom, 30), f.bank.GetSupply(ctx, sdk.DefaultBondDenom))
		queued, err := f.staking.GetUBDQueueTimeSlice(ctx, ctx.BlockTime())
		require.NoError(t, err)
		require.Empty(t, queued)
		message, broken := bankkeeper.TotalSupply(f.bank)(ctx)
		require.False(t, broken, message)
		message, broken = stakingkeeper.ModuleAccountInvariants(f.staking)(ctx)
		require.False(t, broken, message)
		ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(ctx.BlockTime().Add(time.Second)).WithEventManager(sdk.NewEventManager())
	}

	// A later completion must not recover or burn the discarded claim again.
	paid, err := f.staking.CompleteUnbonding(ctx, attacker, val)
	require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
	require.Nil(t, paid)
	require.True(t, f.bank.GetAllBalances(ctx, pool).Empty())
	require.True(t, f.bank.GetAllBalances(ctx, attacker).Empty())
	_, err = f.staking.GetUnbondingDelegationByUnbondingID(ctx, original.Entries[0].UnbondingId)
	require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
	remaining, err := f.staking.DequeueAllMatureUBDQueue(ctx, ctx.BlockTime())
	require.NoError(t, err)
	require.Empty(t, remaining)
	message, broken := bankkeeper.TotalSupply(f.bank)(ctx)
	require.False(t, broken, message)
}

func TestCompleteUnbondingDiscardsClaimOnAnyPayoutError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "unknown address", err: sdkerrors.ErrUnknownAddress},
		{name: "insufficient funds", err: sdkerrors.ErrInsufficientFunds},
		{name: "other error", err: errors.New("payout rejected")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := new(KeeperTestSuite)
			s.SetT(t)
			s.SetupTest()
			dels, vals := createValAddrs(1)
			del, val := dels[0], vals[0]
			ubd, err := s.stakingKeeper.SetUnbondingDelegationEntry(s.ctx, del, val, 1, s.ctx.BlockTime(), math.NewInt(10))
			require.NoError(t, err)
			coins := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10))
			s.bankKeeper.EXPECT().UndelegateCoinsFromModuleToAccount(gomock.Not(s.ctx), stakingtypes.NotBondedPoolName, del, coins).Return(tc.err)
			s.bankKeeper.EXPECT().BurnCoins(gomock.Not(s.ctx), stakingtypes.NotBondedPoolName, coins).Return(nil)

			paid, err := s.stakingKeeper.CompleteUnbonding(s.ctx, del, val)
			require.NoError(t, err)
			require.Empty(t, paid)
			_, err = s.stakingKeeper.GetUnbondingDelegation(s.ctx, del, val)
			require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
			_, err = s.stakingKeeper.GetUnbondingDelegationByUnbondingID(s.ctx, ubd.Entries[0].UnbondingId)
			require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
			_, err = s.stakingKeeper.GetUnbondingType(s.ctx, ubd.Entries[0].UnbondingId)
			require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingType)
		})
	}
}

func TestCompleteUnbondingDiscardsOnlyMatureUnheldEntries(t *testing.T) {
	t.Parallel()
	f := newUnbondingFixture(t)
	ctx := f.ctx
	dels, vals := createValAddrs(1)
	del, val := dels[0], vals[0]
	_, err := f.staking.SetUnbondingDelegationEntry(ctx, del, val, 1, ctx.BlockTime(), math.NewInt(10))
	require.NoError(t, err)
	_, err = f.staking.SetUnbondingDelegationEntry(ctx, del, val, 2, ctx.BlockTime().Add(time.Hour), math.NewInt(20))
	require.NoError(t, err)
	ubd, err := f.staking.SetUnbondingDelegationEntry(ctx, del, val, 3, ctx.BlockTime(), math.NewInt(30))
	require.NoError(t, err)
	heldID := ubd.Entries[2].UnbondingId
	require.NoError(t, f.staking.PutUnbondingOnHold(ctx, heldID))
	original, err := f.staking.GetUnbondingDelegation(ctx, del, val)
	require.NoError(t, err)
	require.NoError(t, banktestutil.FundModuleAccount(ctx, f.bank, stakingtypes.NotBondedPoolName,
		sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 60))))
	pool := f.auth.GetModuleAddress(stakingtypes.NotBondedPoolName)

	paid, err := f.staking.CompleteUnbonding(ctx, del, val)
	require.NoError(t, err)
	require.Empty(t, paid)
	stored, err := f.staking.GetUnbondingDelegation(ctx, del, val)
	require.NoError(t, err)
	require.Equal(t, original.Entries[1:], stored.Entries)
	_, err = f.staking.GetUnbondingDelegationByUnbondingID(ctx, original.Entries[0].UnbondingId)
	require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
	_, err = f.staking.GetUnbondingType(ctx, original.Entries[0].UnbondingId)
	require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingType)
	for _, entry := range stored.Entries {
		_, err := f.staking.GetUnbondingDelegationByUnbondingID(ctx, entry.UnbondingId)
		require.NoError(t, err)
		_, err = f.staking.GetUnbondingType(ctx, entry.UnbondingId)
		require.NoError(t, err)
	}
	require.Equal(t, sdk.NewInt64Coin(sdk.DefaultBondDenom, 50), f.bank.GetBalance(ctx, pool, sdk.DefaultBondDenom))
	require.Equal(t, sdk.NewInt64Coin(sdk.DefaultBondDenom, 50), f.bank.GetSupply(ctx, sdk.DefaultBondDenom))
	message, broken := stakingkeeper.ModuleAccountInvariants(f.staking)(ctx)
	require.False(t, broken, message)

	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(2 * time.Hour))
	paid, err = f.staking.CompleteUnbonding(ctx, del, val)
	require.NoError(t, err)
	require.Empty(t, paid)
	stored, err = f.staking.GetUnbondingDelegationByUnbondingID(ctx, heldID)
	require.NoError(t, err)
	require.Equal(t, original.Entries[2:], stored.Entries)
	require.NoError(t, f.staking.UnbondingCanComplete(ctx, heldID))
	_, err = f.staking.GetUnbondingDelegation(ctx, del, val)
	require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
	_, err = f.staking.GetUnbondingType(ctx, heldID)
	require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingType)
	require.True(t, f.bank.GetAllBalances(ctx, pool).Empty())
	require.True(t, f.bank.GetSupply(ctx, sdk.DefaultBondDenom).IsZero())
	message, broken = bankkeeper.TotalSupply(f.bank)(ctx)
	require.False(t, broken, message)
	message, broken = stakingkeeper.ModuleAccountInvariants(f.staking)(ctx)
	require.False(t, broken, message)
}

func TestDiscardUnbondingRollsBackBurnOnClaimWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		held bool
	}{{name: "ordinary"}, {name: "held", held: true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newUnbondingFixture(t)
			ctx := f.ctx
			dels, vals := createValAddrs(1)
			del, val := dels[0], vals[0]
			ubd, err := f.staking.SetUnbondingDelegationEntry(ctx, del, val, 1, ctx.BlockTime(), math.NewInt(10))
			require.NoError(t, err)
			id := ubd.Entries[0].UnbondingId
			if tc.held {
				require.NoError(t, f.staking.PutUnbondingOnHold(ctx, id))
			}
			original, err := f.staking.GetUnbondingDelegation(ctx, del, val)
			require.NoError(t, err)
			coins := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10))
			require.NoError(t, banktestutil.FundModuleAccount(ctx, f.bank, stakingtypes.NotBondedPoolName, coins))
			pool := f.auth.GetModuleAddress(stakingtypes.NotBondedPoolName)
			events := append(sdk.Events(nil), ctx.EventManager().Events()...)
			updates := f.bank.GetAllTransientAccountBalanceUpdates(ctx)
			complete := func() error {
				if tc.held {
					return f.staking.UnbondingCanComplete(ctx, id)
				}
				_, err := f.staking.CompleteUnbonding(ctx, del, val)
				return err
			}
			f.store.failDelete = stakingtypes.GetUBDKey(del, val)

			require.ErrorIs(t, complete(), errUnbondingWrite)
			require.Equal(t, coins, f.bank.GetAllBalances(ctx, pool))
			require.Equal(t, coins[0], f.bank.GetSupply(ctx, coins[0].Denom))
			stored, err := f.staking.GetUnbondingDelegationByUnbondingID(ctx, id)
			require.NoError(t, err)
			require.Equal(t, original, stored)
			_, err = f.staking.GetUnbondingType(ctx, id)
			require.NoError(t, err)
			require.Equal(t, events, ctx.EventManager().Events())
			require.Equal(t, updates, f.bank.GetAllTransientAccountBalanceUpdates(ctx))

			f.store.failDelete = nil
			require.NoError(t, complete())
			require.True(t, f.bank.GetAllBalances(ctx, pool).Empty())
			require.True(t, f.bank.GetSupply(ctx, coins[0].Denom).IsZero())
			_, err = f.staking.GetUnbondingDelegationByUnbondingID(ctx, id)
			require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
			_, err = f.staking.GetUnbondingType(ctx, id)
			require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingType)
			message, broken := bankkeeper.TotalSupply(f.bank)(ctx)
			require.False(t, broken, message)
			message, broken = stakingkeeper.ModuleAccountInvariants(f.staking)(ctx)
			require.False(t, broken, message)
		})
	}
}
