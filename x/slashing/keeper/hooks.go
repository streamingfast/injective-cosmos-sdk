package keeper

import (
	"context"
	"time"

	"github.com/cometbft/cometbft/crypto"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/slashing/types"
)

var _ types.StakingHooks = Hooks{}

// Hooks wrapper struct for slashing keeper
type Hooks struct {
	k Keeper
}

// Return the slashing hooks
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// AfterValidatorBonded updates the signing info start height or create a new signing info
func (h Hooks) AfterValidatorBonded(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer h.k.Meter(ctx).FuncTiming(&sdkCtx, "AfterValidatorBonded")(&err)
	signingInfo, err := h.k.GetValidatorSigningInfo(sdkCtx, consAddr)
	if err == nil {
		signingInfo.StartHeight = sdkCtx.BlockHeight()
	} else {
		signingInfo = types.NewValidatorSigningInfo(
			consAddr,
			sdkCtx.BlockHeight(),
			0,
			time.Unix(0, 0),
			false,
			0,
		)
	}

	return h.k.SetValidatorSigningInfo(sdkCtx, consAddr, signingInfo)
}

// AfterValidatorRemoved deletes the address-pubkey relation when a validator is removed,
func (h Hooks) AfterValidatorRemoved(ctx context.Context, consAddr sdk.ConsAddress, _ sdk.ValAddress) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer h.k.Meter(ctx).FuncTiming(&sdkCtx, "AfterValidatorRemoved")(&err)

	return h.k.deleteAddrPubkeyRelation(sdkCtx, crypto.Address(consAddr))
}

// AfterValidatorCreated adds the address-pubkey relation when a validator is created.
func (h Hooks) AfterValidatorCreated(ctx context.Context, valAddr sdk.ValAddress) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer h.k.Meter(ctx).FuncTiming(&sdkCtx, "AfterValidatorCreated")(&err)
	validator, err := h.k.sk.Validator(sdkCtx, valAddr)
	if err != nil {
		return err
	}

	consPk, err := validator.ConsPubKey()
	if err != nil {
		return err
	}

	return h.k.AddPubkey(sdkCtx, consPk)
}

func (h Hooks) AfterValidatorBeginUnbonding(_ context.Context, _ sdk.ConsAddress, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeValidatorModified(_ context.Context, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeDelegationCreated(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeDelegationSharesModified(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeDelegationRemoved(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) AfterDelegationModified(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeValidatorSlashed(_ context.Context, _ sdk.ValAddress, _ sdkmath.LegacyDec) error {
	return nil
}

func (h Hooks) AfterUnbondingInitiated(_ context.Context, _ uint64) error {
	return nil
}
