package keeper_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func TestUndelegateCoinsMissingRecipientRollback(t *testing.T) {
	t.Parallel()
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	ctx := sdk.UnwrapSDKContext(s.ctx)
	pool := authtypes.NewEmptyModuleAccount("atomic_pool", authtypes.Minter, authtypes.Staking)
	recipient := sdk.AccAddress([]byte("missing_recipient___"))
	coins := sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
	s.mockMintCoins(pool)
	require.NoError(t, s.bankKeeper.MintCoins(ctx, pool.Name, coins))
	s.authKeeper.EXPECT().GetAccount(gomock.Any(), pool.GetAddress()).Return(pool).AnyTimes()
	s.authKeeper.EXPECT().GetAccount(gomock.Any(), recipient).Return(nil).AnyTimes()
	events := append(sdk.Events(nil), ctx.EventManager().Events()...)
	updates := s.bankKeeper.GetAllTransientAccountBalanceUpdates(ctx)

	for range 3 {
		err := s.bankKeeper.UndelegateCoins(ctx, pool.GetAddress(), recipient, coins)
		require.ErrorIs(t, err, sdkerrors.ErrUnknownAddress)
		require.Equal(t, coins, s.bankKeeper.GetAllBalances(ctx, pool.GetAddress()))
		require.True(t, s.bankKeeper.GetAllBalances(ctx, recipient).Empty())
		require.Equal(t, coins[0], s.bankKeeper.GetSupply(ctx, coins[0].Denom))
		require.Equal(t, events, ctx.EventManager().Events())
		require.Equal(t, updates, s.bankKeeper.GetAllTransientAccountBalanceUpdates(ctx))
	}
}
