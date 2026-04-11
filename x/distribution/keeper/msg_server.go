package keeper

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-metrics"

	"cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

type msgServer struct {
	Keeper
}

var _ types.MsgServer = msgServer{}

// NewMsgServerImpl returns an implementation of the distribution MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

func (k msgServer) SetWithdrawAddress(ctx context.Context, msg *types.MsgSetWithdrawAddress) (meterResult *types.MsgSetWithdrawAddressResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Keeper.Meter(ctx).FuncTiming(&sdkCtx, "SetWithdrawAddress")(&err)

	delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(msg.DelegatorAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid delegator address: %s", err)
	}

	withdrawAddress, err := k.authKeeper.AddressCodec().StringToBytes(msg.WithdrawAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid withdraw address: %s", err)
	}

	err = k.SetWithdrawAddr(sdkCtx, delegatorAddress, withdrawAddress)
	if err != nil {
		return nil, err
	}

	return &types.MsgSetWithdrawAddressResponse{}, nil
}

func (k msgServer) WithdrawDelegatorReward(ctx context.Context, msg *types.MsgWithdrawDelegatorReward) (meterResult *types.MsgWithdrawDelegatorRewardResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Keeper.Meter(ctx).FuncTiming(&sdkCtx, "WithdrawDelegatorReward")(&err)

	valAddr, err := k.stakingKeeper.ValidatorAddressCodec().StringToBytes(msg.ValidatorAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid validator address: %s", err)
	}

	delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(msg.DelegatorAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid delegator address: %s", err)
	}

	amount, err := k.WithdrawDelegationRewards(sdkCtx, delegatorAddress, valAddr)
	if err != nil {
		return nil, err
	}

	defer func() {
		for _, a := range amount {
			if a.Amount.IsInt64() {
				telemetry.SetGaugeWithLabels(
					[]string{"tx", "msg", "withdraw_reward"},
					float32(a.Amount.Int64()),
					[]metrics.Label{telemetry.NewLabel("denom", a.Denom)},
				)
			}
		}
	}()

	return &types.MsgWithdrawDelegatorRewardResponse{Amount: amount}, nil
}

func (k msgServer) WithdrawValidatorCommission(ctx context.Context, msg *types.MsgWithdrawValidatorCommission) (meterResult *types.MsgWithdrawValidatorCommissionResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Keeper.Meter(ctx).FuncTiming(&sdkCtx, "WithdrawValidatorCommission")(&err)

	valAddr, err := k.stakingKeeper.ValidatorAddressCodec().StringToBytes(msg.ValidatorAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid validator address: %s", err)
	}

	amount, err := k.Keeper.WithdrawValidatorCommission(sdkCtx, valAddr)
	if err != nil {
		return nil, err
	}

	defer func() {
		for _, a := range amount {
			if a.Amount.IsInt64() {
				telemetry.SetGaugeWithLabels(
					[]string{"tx", "msg", "withdraw_commission"},
					float32(a.Amount.Int64()),
					[]metrics.Label{telemetry.NewLabel("denom", a.Denom)},
				)
			}
		}
	}()

	return &types.MsgWithdrawValidatorCommissionResponse{Amount: amount}, nil
}

func (k msgServer) FundCommunityPool(ctx context.Context, msg *types.MsgFundCommunityPool) (meterResult *types.MsgFundCommunityPoolResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Keeper.Meter(ctx).FuncTiming(&sdkCtx, "FundCommunityPool")(&err)

	depositor, err := k.authKeeper.AddressCodec().StringToBytes(msg.Depositor)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid depositor address: %s", err)
	}

	if err = validateAmount(msg.Amount); err != nil {
		return nil, err
	}

	if err = k.Keeper.FundCommunityPool(sdkCtx, msg.Amount, depositor); err != nil {
		return nil, err
	}

	return &types.MsgFundCommunityPoolResponse{}, nil
}

func (k msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (meterResult *types.MsgUpdateParamsResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Keeper.Meter(ctx).FuncTiming(&sdkCtx, "UpdateParams")(&err)

	if err = k.validateAuthority(msg.Authority); err != nil {
		return nil, err
	}

	if (!msg.Params.BaseProposerReward.IsNil() && !msg.Params.BaseProposerReward.IsZero()) || //nolint:staticcheck // deprecated but kept for backwards compatibility
		(!msg.Params.BonusProposerReward.IsNil() && !msg.Params.BonusProposerReward.IsZero()) { //nolint:staticcheck // deprecated but kept for backwards compatibility
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "cannot update base or bonus proposer reward because these are deprecated fields")
	}

	if err = msg.Params.ValidateBasic(); err != nil {
		return nil, err
	}

	if err = k.Params.Set(sdkCtx, msg.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

func (k msgServer) CommunityPoolSpend(ctx context.Context, msg *types.MsgCommunityPoolSpend) (meterResult *types.MsgCommunityPoolSpendResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Keeper.Meter(ctx).FuncTiming(&sdkCtx, "CommunityPoolSpend")(&err)

	if err = k.validateAuthority(msg.Authority); err != nil {
		return nil, err
	}

	if err = validateAmount(msg.Amount); err != nil {
		return nil, err
	}

	recipient, err := k.authKeeper.AddressCodec().StringToBytes(msg.Recipient)
	if err != nil {
		return nil, err
	}

	if k.bankKeeper.BlockedAddr(recipient) {
		return nil, errors.Wrapf(sdkerrors.ErrUnauthorized, "%s is not allowed to receive external funds", msg.Recipient)
	}

	if err = k.DistributeFromFeePool(sdkCtx, msg.Amount, recipient); err != nil {
		return nil, err
	}

	logger := k.Logger(sdkCtx)
	logger.Info("transferred from the community pool to recipient", "amount", msg.Amount.String(), "recipient", msg.Recipient)

	return &types.MsgCommunityPoolSpendResponse{}, nil
}

func (k msgServer) DepositValidatorRewardsPool(ctx context.Context, msg *types.MsgDepositValidatorRewardsPool) (meterResult *types.MsgDepositValidatorRewardsPoolResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Keeper.Meter(ctx).FuncTiming(&sdkCtx, "DepositValidatorRewardsPool")(&err)

	depositor, err := k.authKeeper.AddressCodec().StringToBytes(msg.Depositor)
	if err != nil {
		return nil, err
	}

	bondDenom, err := k.stakingKeeper.BondDenom(sdkCtx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get bond denom")
	}

	for _, coin := range msg.Amount {
		if coin.Denom != bondDenom {
			return nil, errors.Wrapf(sdkerrors.ErrInvalidCoins, "not a bond token denom: %s", coin.Denom)
		}
	}

	// deposit coins from depositor's account to the distribution module
	if err = k.bankKeeper.SendCoinsFromAccountToModule(sdkCtx, depositor, types.ModuleName, msg.Amount); err != nil {
		return nil, err
	}

	valAddr, err := k.stakingKeeper.ValidatorAddressCodec().StringToBytes(msg.ValidatorAddress)
	if err != nil {
		return nil, err
	}

	validator, err := k.stakingKeeper.Validator(sdkCtx, valAddr)
	if err != nil {
		return nil, err
	}

	if validator == nil {
		return nil, errors.Wrapf(types.ErrNoValidatorExists, msg.ValidatorAddress)
	}

	// Allocate tokens from the distribution module to the validator, which are
	// then distributed to the validator's delegators.
	reward := sdk.NewDecCoinsFromCoins(msg.Amount...)
	if err = k.AllocateTokensToValidator(sdkCtx, validator, reward); err != nil {
		return nil, err
	}

	// make sure the reward pool isn't already full.
	if !validator.GetTokens().IsZero() {
		rewards, err := k.GetValidatorCurrentRewards(sdkCtx, valAddr)
		if err != nil {
			return nil, err
		}
		current := rewards.Rewards
		historical, err := k.GetValidatorHistoricalRewards(sdkCtx, valAddr, rewards.Period-1)
		if err != nil {
			return nil, err
		}
		if !historical.CumulativeRewardRatio.IsZero() {
			rewardRatio := historical.CumulativeRewardRatio
			var panicErr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicErr = fmt.Errorf("deposit is too large: %v", r)
					}
				}()
				rewardRatio.Add(current...)
			}()

			// Check if the deferred function caught a panic
			if panicErr != nil {
				return nil, fmt.Errorf("unable to deposit coins: %w", panicErr)
			}
		}
	}

	logger := k.Logger(sdkCtx)
	logger.Info(
		"transferred from rewards to validator rewards pool",
		"depositor", msg.Depositor,
		"amount", msg.Amount.String(),
		"validator", msg.ValidatorAddress,
	)

	return &types.MsgDepositValidatorRewardsPoolResponse{}, nil
}

func (k *Keeper) validateAuthority(authority string) error {
	if _, err := k.authKeeper.AddressCodec().StringToBytes(authority); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("invalid authority address: %s", err)
	}

	if k.authority != authority {
		return errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", k.authority, authority)
	}

	return nil
}

func validateAmount(amount sdk.Coins) error {
	if amount == nil {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, "amount cannot be nil")
	}

	if err := amount.Validate(); err != nil {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, amount.String())
	}

	return nil
}
