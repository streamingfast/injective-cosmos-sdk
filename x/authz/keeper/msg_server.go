package keeper

import (
	"context"
	"errors"
	"strings"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var _ authz.MsgServer = Keeper{}

// Grant implements the MsgServer.Grant method to create a new grant.
func (k Keeper) Grant(goCtx context.Context, msg *authz.MsgGrant) (meterResult *authz.MsgGrantResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(goCtx).FuncTiming(&sdkCtx, "Grant")(&err)

	if strings.EqualFold(msg.Grantee, msg.Granter) {
		return nil, authz.ErrGranteeIsGranter
	}

	grantee, err := k.authKeeper.AddressCodec().StringToBytes(msg.Grantee)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid grantee address: %s", err)
	}

	granter, err := k.authKeeper.AddressCodec().StringToBytes(msg.Granter)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid granter address: %s", err)
	}

	if err = msg.Grant.ValidateBasic(); err != nil {
		return nil, err
	}

	// create the account if it is not in account state

	granteeAcc := k.authKeeper.GetAccount(sdkCtx, grantee)
	if granteeAcc == nil {
		if k.bankKeeper.BlockedAddr(grantee) {
			return nil, sdkerrors.ErrUnauthorized.Wrapf("%s is not allowed to receive funds", grantee)
		}

		granteeAcc = k.authKeeper.NewAccountWithAddress(sdkCtx, grantee)
		k.authKeeper.SetAccount(sdkCtx, granteeAcc)
	}

	authorization, err := msg.GetAuthorization()
	if err != nil {
		return nil, err
	}

	t := authorization.MsgTypeURL()
	if k.router.HandlerByTypeURL(t) == nil {
		return nil, sdkerrors.ErrInvalidType.Wrapf("%s doesn't exist.", t)
	}

	// check if granter or grantee are blacklisted for enforced restrictions denoms
	if sendAuth, ok := authorization.(*banktypes.SendAuthorization); k.permissionsKeeper != nil && ok {
		for _, denom := range sendAuth.SpendLimit.Denoms() {
			if k.permissionsKeeper.IsEnforcedRestrictionsDenom(sdkCtx, denom) {
				fakeAmount := sdk.NewCoin(denom, math.ZeroInt())
				if _, err := k.permissionsKeeper.SendRestrictionFn(sdkCtx, granter, grantee, fakeAmount); err != nil {
					return nil, sdkerrors.ErrUnauthorized.Wrapf("permissions check failed for enforced denom %s: %s", denom, err.Error())
				}
			}
		}
	}

	err = k.SaveGrant(sdkCtx, grantee, granter, authorization, msg.Grant.Expiration)
	if err != nil {
		return nil, err
	}

	return &authz.MsgGrantResponse{}, nil
}

// Revoke implements the MsgServer.Revoke method.
func (k Keeper) Revoke(goCtx context.Context, msg *authz.MsgRevoke) (meterResult *authz.MsgRevokeResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(goCtx).FuncTiming(&sdkCtx, "Revoke")(&err)

	if strings.EqualFold(msg.Grantee, msg.Granter) {
		return nil, authz.ErrGranteeIsGranter
	}

	grantee, err := k.authKeeper.AddressCodec().StringToBytes(msg.Grantee)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid grantee address: %s", err)
	}

	granter, err := k.authKeeper.AddressCodec().StringToBytes(msg.Granter)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid granter address: %s", err)
	}

	if msg.MsgTypeUrl == "" {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("missing msg method name")
	}

	if err = k.DeleteGrant(sdkCtx, grantee, granter, msg.MsgTypeUrl); err != nil {
		return nil, err
	}

	return &authz.MsgRevokeResponse{}, nil
}

// Exec implements the MsgServer.Exec method.
func (k Keeper) Exec(goCtx context.Context, msg *authz.MsgExec) (meterResult *authz.MsgExecResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(goCtx).FuncTiming(&sdkCtx, "Exec")(&err)

	if msg.Grantee == "" {
		return nil, errors.New("empty address string is not allowed")
	}

	grantee, err := k.authKeeper.AddressCodec().StringToBytes(msg.Grantee)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid grantee address: %s", err)
	}

	if len(msg.Msgs) == 0 {
		return nil, sdkerrors.ErrInvalidRequest.Wrapf("messages cannot be empty")
	}

	msgs, err := msg.GetMessages()
	if err != nil {
		return nil, err
	}

	if err = validateMsgs(msgs); err != nil {
		return nil, err
	}

	results, err := k.DispatchActions(sdkCtx, grantee, msgs)
	if err != nil {
		return nil, err
	}

	return &authz.MsgExecResponse{Results: results}, nil
}

func validateMsgs(msgs []sdk.Msg) error {
	for i, msg := range msgs {
		m, ok := msg.(sdk.HasValidateBasic)
		if !ok {
			continue
		}

		if err := m.ValidateBasic(); err != nil {
			return errorsmod.Wrapf(err, "msg %d", i)
		}
	}

	return nil
}

// ExecCompat implements the MsgServer.ExecCompat method.
// Deprecated: This method is deprecated and disabled. It will be removed in a future version.
func (k Keeper) ExecCompat(goCtx context.Context, msg *authz.MsgExecCompat) (meterResult *authz.MsgExecCompatResponse, err error) {
	sdkCtx := sdk.UnwrapSDKContext(goCtx)
	defer k.Meter(goCtx).FuncTiming(&sdkCtx, "ExecCompat")(&err)

	return nil, sdkerrors.ErrInvalidRequest.Wrap("MsgExecCompat is deprecated and has been disabled")
}
