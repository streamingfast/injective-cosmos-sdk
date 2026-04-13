package keeper

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/gogoproto/proto"

	corestoretypes "cosmossdk.io/core/store"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/InjectiveLabs/metrics/v2"
)

// TODO: Revisit this once we have propoer gas fee framework.
// Tracking issues https://github.com/cosmos/cosmos-sdk/issues/9054,
// https://github.com/cosmos/cosmos-sdk/discussions/9072
const gasCostPerIteration = uint64(20)

type Keeper struct {
	storeService      corestoretypes.KVStoreService
	cdc               codec.Codec
	router            baseapp.MessageRouter
	authKeeper        authz.AccountKeeper
	bankKeeper        authz.BankKeeper
	permissionsKeeper authz.PermissionsKeeper
	meter             metrics.Meter
}

// NewKeeper constructs a message authorization Keeper
func NewKeeper(storeService corestoretypes.KVStoreService, cdc codec.Codec, router baseapp.MessageRouter, ak authz.AccountKeeper) Keeper {
	return Keeper{
		storeService: storeService,
		cdc:          cdc,
		router:       router,
		authKeeper:   ak,
	}
}

// Super ugly hack to not be breaking in v0.50 and v0.47
// DO NOT USE.
func (k Keeper) SetBankKeeper(bk authz.BankKeeper) Keeper {
	k.bankKeeper = bk
	return k
}

func (k *Keeper) SetPermissionsKeeper(pk authz.PermissionsKeeper) {
	k.permissionsKeeper = pk
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", fmt.Sprintf("x/%s", authz.ModuleName))
}

// getGrant returns grant stored at skey.
func (k Keeper) getGrant(ctx context.Context, skey []byte) (grant authz.Grant, found bool) {
	var err error
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "getGrant")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)

	bz, err := store.Get(skey)
	if err != nil {
		panic(err)
	}

	if bz == nil {
		return grant, false
	}
	k.cdc.MustUnmarshal(bz, &grant)
	return grant, true
}

func (k Keeper) update(ctx context.Context, grantee, granter sdk.AccAddress, updated authz.Authorization) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "update")(&err)

	skey := grantStoreKey(grantee, granter, updated.MsgTypeURL())
	grant, found := k.getGrant(sdkCtx, skey)
	if !found {
		return authz.ErrNoAuthorizationFound
	}

	msg, ok := updated.(proto.Message)
	if !ok {
		return sdkerrors.ErrPackAny.Wrapf("cannot proto marshal %T", updated)
	}

	any, err := codectypes.NewAnyWithValue(msg)
	if err != nil {
		return err
	}

	grant.Authorization = any
	store := k.storeService.OpenKVStore(sdkCtx)
	store.Set(skey, k.cdc.MustMarshal(&grant))

	return nil
}

// DispatchActions attempts to execute the provided messages via authorization
// grants from the message signer to the grantee.
func (k Keeper) DispatchActions(ctx context.Context, grantee sdk.AccAddress, msgs []sdk.Msg) (meterResult [][]byte, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "DispatchActions")(&err)

	results := make([][]byte, len(msgs))
	now := sdkCtx.BlockTime()

	for i, msg := range msgs {
		signers, _, err := k.cdc.GetMsgV1Signers(msg)
		if err != nil {
			return nil, err
		}

		if len(signers) != 1 {
			return nil, authz.ErrAuthorizationNumOfSigners
		}

		granter := signers[0]

		// If granter != grantee then check authorization.Accept, otherwise we
		// implicitly accept.
		if !bytes.Equal(granter, grantee) {
			skey := grantStoreKey(grantee, granter, sdk.MsgTypeURL(msg))

			grant, found := k.getGrant(sdkCtx, skey)
			if !found {
				return nil, errorsmod.Wrapf(authz.ErrNoAuthorizationFound,
					"failed to get grant with given granter: %s, grantee: %s & msgType: %s ", sdk.AccAddress(granter), grantee, sdk.MsgTypeURL(msg))
			}

			if grant.Expiration != nil && grant.Expiration.Before(now) {
				return nil, authz.ErrAuthorizationExpired
			}

			authorization, err := grant.GetAuthorization()
			if err != nil {
				return nil, err
			}

			resp, err := authorization.Accept(sdkCtx, msg)
			if err != nil {
				return nil, err
			}

			if resp.Delete {
				err = k.DeleteGrant(sdkCtx, grantee, granter, sdk.MsgTypeURL(msg))
			} else if resp.Updated != nil {
				err = k.update(sdkCtx, grantee, granter, resp.Updated)
			}
			if err != nil {
				return nil, err
			}

			if !resp.Accept {
				return nil, sdkerrors.ErrUnauthorized
			}
		}

		handler := k.router.Handler(msg)
		if handler == nil {
			return nil, sdkerrors.ErrUnknownRequest.Wrapf("unrecognized message route: %s", sdk.MsgTypeURL(msg))
		}

		msgResp, err := handler(sdkCtx, msg)
		if err != nil {
			return nil, errorsmod.Wrapf(err, "failed to execute message; message %v", msg)
		}

		results[i] = msgResp.Data

		// emit the events from the dispatched actions
		events := msgResp.Events
		sdkEvents := make([]sdk.Event, 0, len(events))
		for _, event := range events {
			e := event
			e.Attributes = append(e.Attributes, abci.EventAttribute{Key: "authz_msg_index", Value: strconv.Itoa(i)})

			sdkEvents = append(sdkEvents, sdk.Event(e))
		}

		sdkCtx.EventManager().EmitEvents(sdkEvents)
	}

	return results, nil
}

// SaveGrant method grants the provided authorization to the grantee on the granter's account
// with the provided expiration time and insert authorization key into the grants queue. If there is an existing authorization grant for the
// same `sdk.Msg` type, this grant overwrites that.
func (k Keeper) SaveGrant(ctx context.Context, grantee, granter sdk.AccAddress, authorization authz.Authorization, expiration *time.Time) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "SaveGrant")(&err)
	msgType := authorization.MsgTypeURL()
	store := k.storeService.OpenKVStore(sdkCtx)
	skey := grantStoreKey(grantee, granter, msgType)

	grant, err := authz.NewGrant(sdkCtx.BlockTime(), authorization, expiration)
	if err != nil {
		return err
	}

	var oldExp *time.Time
	if oldGrant, found := k.getGrant(sdkCtx, skey); found {
		oldExp = oldGrant.Expiration
	}

	if oldExp != nil && (expiration == nil || !oldExp.Equal(*expiration)) {
		if err = k.removeFromGrantQueue(sdkCtx, skey, granter, grantee, *oldExp); err != nil {
			return err
		}
	}

	// If the expiration didn't change, then we don't remove it and we should not insert again
	if expiration != nil && (oldExp == nil || !oldExp.Equal(*expiration)) {
		if err = k.insertIntoGrantQueue(sdkCtx, granter, grantee, msgType, *expiration); err != nil {
			return err
		}
	}

	// store index
	indexKey := grantsByGranteeAndMsgTypeAndGranterKey(grantee, msgType, granter)
	err = store.Set(indexKey, []byte{})
	if err != nil {
		return err
	}

	bz, err := k.cdc.Marshal(&grant)
	if err != nil {
		return err
	}

	err = store.Set(skey, bz)
	if err != nil {
		return err
	}

	return sdkCtx.EventManager().EmitTypedEvent(&authz.EventGrant{
		MsgTypeUrl: authorization.MsgTypeURL(),
		Granter:    granter.String(),
		Grantee:    grantee.String(),
	})
}

// DeleteGrant revokes any authorization for the provided message type granted to the grantee
// by the granter.
func (k Keeper) DeleteGrant(ctx context.Context, grantee, granter sdk.AccAddress, msgType string) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "DeleteGrant")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	skey := grantStoreKey(grantee, granter, msgType)
	grant, found := k.getGrant(sdkCtx, skey)
	if !found {
		return errorsmod.Wrapf(authz.ErrNoAuthorizationFound, "failed to delete grant with key %s", hex.EncodeToString(skey))
	}

	if grant.Expiration != nil {
		err = k.removeFromGrantQueue(sdkCtx, skey, granter, grantee, *grant.Expiration)
		if err != nil {
			return err
		}
	}

	// delete index
	indexKey := grantsByGranteeAndMsgTypeAndGranterKey(grantee, msgType, granter)
	err = store.Delete(indexKey)
	if err != nil {
		return err
	}
	err = store.Delete(skey)
	if err != nil {
		return err
	}
	return sdkCtx.EventManager().EmitTypedEvent(&authz.EventRevoke{
		MsgTypeUrl: msgType,
		Granter:    granter.String(),
		Grantee:    grantee.String(),
	})
}

// GetAuthorizations Returns list of `Authorizations` granted to the grantee by the granter.
func (k Keeper) GetAuthorizations(ctx context.Context, grantee, granter sdk.AccAddress) (meterResult []authz.Authorization, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetAuthorizations")(&err)

	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(sdkCtx))
	key := grantStoreKey(grantee, granter, "")
	iter := storetypes.KVStorePrefixIterator(store, key)
	defer iter.Close()

	var authorizations []authz.Authorization
	for ; iter.Valid(); iter.Next() {
		var authorization authz.Grant
		if err = k.cdc.Unmarshal(iter.Value(), &authorization); err != nil {
			return nil, err
		}

		a, err := authorization.GetAuthorization()
		if err != nil {
			return nil, err
		}

		authorizations = append(authorizations, a)
	}

	return authorizations, nil
}

// GetAuthorization returns an Authorization and it's expiration time.
// A nil Authorization is returned under the following circumstances:
//   - No grant is found.
//   - A grant is found, but it is expired.
//   - There was an error getting the authorization from the grant.
func (k Keeper) GetAuthorization(ctx context.Context, grantee, granter sdk.AccAddress, msgType string) (authz.Authorization, *time.Time) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "GetAuthorization")()
	grant, found := k.getGrant(sdkCtx, grantStoreKey(grantee, granter, msgType))
	if !found || (grant.Expiration != nil && grant.Expiration.Before(sdkCtx.BlockHeader().Time)) {
		return nil, nil
	}

	auth, err := grant.GetAuthorization()
	if err != nil {
		return nil, nil
	}

	return auth, grant.Expiration
}

// IterateGrants iterates over all authorization grants
// This function should be used with caution because it can involve significant IO operations.
// It should not be used in query or msg services without charging additional gas.
// The iteration stops when the handler function returns true or the iterator exhaust.
func (k Keeper) IterateGrants(ctx context.Context,
	handler func(granterAddr, granteeAddr sdk.AccAddress, grant authz.Grant) bool,
) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "IterateGrants")()

	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(sdkCtx))
	iter := storetypes.KVStorePrefixIterator(store, GrantKey)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var grant authz.Grant
		granterAddr, granteeAddr, _ := parseGrantStoreKey(iter.Key())
		k.cdc.MustUnmarshal(iter.Value(), &grant)
		if handler(granterAddr, granteeAddr, grant) {
			break
		}
	}
}

func (k Keeper) getGrantQueueItem(ctx context.Context, expiration time.Time, granter, grantee sdk.AccAddress) (meterResult *authz.GrantQueueItem, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "getGrantQueueItem")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	bz, err := store.Get(GrantQueueKey(expiration, granter, grantee))
	if err != nil {
		return nil, err
	}

	if bz == nil {
		return &authz.GrantQueueItem{}, nil
	}

	var queueItems authz.GrantQueueItem
	if err = k.cdc.Unmarshal(bz, &queueItems); err != nil {
		return nil, err
	}
	return &queueItems, nil
}

func (k Keeper) setGrantQueueItem(ctx context.Context, expiration time.Time,
	granter, grantee sdk.AccAddress, queueItems *authz.GrantQueueItem,
) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "setGrantQueueItem")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	bz, err := k.cdc.Marshal(queueItems)
	if err != nil {
		return err
	}
	return store.Set(GrantQueueKey(expiration, granter, grantee), bz)
}

// insertIntoGrantQueue inserts a grant key into the grant queue
func (k Keeper) insertIntoGrantQueue(ctx context.Context, granter, grantee sdk.AccAddress, msgType string, expiration time.Time) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "insertIntoGrantQueue")(&err)

	queueItems, err := k.getGrantQueueItem(sdkCtx, expiration, granter, grantee)
	if err != nil {
		return err
	}

	queueItems.MsgTypeUrls = append(queueItems.MsgTypeUrls, msgType)
	return k.setGrantQueueItem(sdkCtx, expiration, granter, grantee, queueItems)
}

// removeFromGrantQueue removes a grant key from the grant queue
func (k Keeper) removeFromGrantQueue(ctx context.Context, grantKey []byte, granter, grantee sdk.AccAddress, expiration time.Time) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "removeFromGrantQueue")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)
	key := GrantQueueKey(expiration, granter, grantee)
	bz, err := store.Get(key)
	if err != nil {
		return err
	}

	if bz == nil {
		return errorsmod.Wrap(authz.ErrNoGrantKeyFound, "can't remove grant from the expire queue, grant key not found")
	}

	var queueItem authz.GrantQueueItem
	if err = k.cdc.Unmarshal(bz, &queueItem); err != nil {
		return err
	}

	_, _, msgType := parseGrantStoreKey(grantKey)
	queueItems := queueItem.MsgTypeUrls

	for index, typeURL := range queueItems {
		sdkCtx.GasMeter().ConsumeGas(gasCostPerIteration, "grant queue")

		if typeURL == msgType {
			end := len(queueItem.MsgTypeUrls) - 1
			queueItems[index] = queueItems[end]
			queueItems = queueItems[:end]

			if err = k.setGrantQueueItem(sdkCtx, expiration, granter, grantee, &authz.GrantQueueItem{
				MsgTypeUrls: queueItems,
			}); err != nil {
				return err
			}
			break
		}
	}

	return nil
}

// DequeueAndDeleteExpiredGrants deletes expired grants from the state and grant queue.
func (k Keeper) DequeueAndDeleteExpiredGrants(ctx context.Context) (err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "DequeueAndDeleteExpiredGrants")(&err)

	store := k.storeService.OpenKVStore(sdkCtx)

	iterator, err := store.Iterator(GrantQueuePrefix, storetypes.InclusiveEndBytes(GrantQueueTimePrefix(sdkCtx.BlockTime())))
	if err != nil {
		return err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var queueItem authz.GrantQueueItem
		if err = k.cdc.Unmarshal(iterator.Value(), &queueItem); err != nil {
			return err
		}

		_, granter, grantee, err := parseGrantQueueKey(iterator.Key())
		if err != nil {
			return err
		}

		err = store.Delete(iterator.Key())
		if err != nil {
			return err
		}

		for _, typeURL := range queueItem.MsgTypeUrls {
			err = store.Delete(grantStoreKey(grantee, granter, typeURL))
			if err != nil {
				return err
			}
			err = store.Delete(grantsByGranteeAndMsgTypeAndGranterKey(grantee, typeURL, granter))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// getAllGranterAuthorizations Returns list of grantees that the granter have given msgType authorizations to.
func (k Keeper) getAllGranterAuthorizations(c context.Context, granter sdk.AccAddress, msgType string) ([]sdk.AccAddress, error) {
	ctx := sdk.UnwrapSDKContext(c)
	store := k.storeService.OpenKVStore(ctx)

	key := grantStorePrefix(granter)

	iterator, err := store.Iterator(key, storetypes.PrefixEndBytes(key))
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	var grantees []sdk.AccAddress

	for ; iterator.Valid(); iterator.Next() {
		_, grantee, authMsgType := parseGrantStoreKey(iterator.Key())

		if authMsgType != msgType {
			continue
		}

		grantees = append(grantees, grantee)
	}

	return grantees, nil
}

func (k Keeper) getGrantersOfGranteeForMsgType(c context.Context, grantee sdk.AccAddress, msgType string) ([]sdk.AccAddress, error) {
	ctx := sdk.UnwrapSDKContext(c)
	store := k.storeService.OpenKVStore(ctx)

	key := grantsByGranteeAndMsgTypePrefix(grantee, msgType)

	iterator, err := store.Iterator(key, storetypes.PrefixEndBytes(key))
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	var granters []sdk.AccAddress

	for ; iterator.Valid(); iterator.Next() {
		_, _, granter := parseIndexByGranteeAndMsgTypeKey(iterator.Key())
		granters = append(granters, granter)
	}

	return granters, nil
}

// OnEnforcedRestrictionRemoveAuthorizations removes bank send authorization issued by userAddr as granter,
// and to userAddr as a grantee. To be used in callbacks for handling blacklisting by permissions module.
func (k Keeper) OnEnforcedRestrictionRemoveAuthorizations(ctx sdk.Context, userAddr sdk.AccAddress) (err error) {
	defer k.Meter(ctx).FuncTiming(&ctx, "OnEnforcedRestrictionRemoveAuthorizations")(&err)

	msgTypeURL := banktypes.SendAuthorization{}.MsgTypeURL()

	// delete user as granter authorization
	grantees, err := k.getAllGranterAuthorizations(ctx, userAddr, msgTypeURL)
	if err != nil {
		return errorsmod.Wrapf(err, "can't get grantees for granter %s to remove authorizations", userAddr)
	}
	for _, grantee := range grantees {
		if err := k.DeleteGrant(ctx, grantee, userAddr, msgTypeURL); err != nil {
			return errorsmod.Wrap(err, "can't delete grant")
		}
	}

	// delete user as grantee authorizations
	granters, err := k.getGrantersOfGranteeForMsgType(ctx, userAddr, msgTypeURL)
	if err != nil {
		return errorsmod.Wrapf(err, "can't get granters for grantee %s to remove authorizations", userAddr)
	}

	for _, granter := range granters {
		if err := k.DeleteGrant(ctx, userAddr, granter, msgTypeURL); err != nil {
			return errorsmod.Wrap(err, "can't delete grant")
		}
	}

	return nil
}

func (k *Keeper) Meter(ctx context.Context) metrics.Meter {
	if k.meter == nil {
		k.meter = sdk.UnwrapSDKContext(ctx).Meter().SubMeter(authz.ModuleName, metrics.Tag("svc", authz.ModuleName))
	}

	return k.meter
}
