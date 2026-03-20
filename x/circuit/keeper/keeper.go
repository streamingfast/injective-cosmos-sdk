package keeper

import (
	context "context"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/store"
	"cosmossdk.io/x/circuit/types"
	"github.com/InjectiveLabs/metrics/v2"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Keeper defines the circuit module's keeper.
type Keeper struct {
	meter        metrics.Meter
	cdc          codec.BinaryCodec
	storeService store.KVStoreService

	authority []byte

	addressCodec address.Codec

	Schema collections.Schema
	// Permissions contains the permissions for each account
	Permissions collections.Map[[]byte, types.Permissions]
	// DisableList contains the message URLs that are disabled
	DisableList collections.KeySet[string]
}

// NewKeeper constructs a new Circuit Keeper instance
func NewKeeper(cdc codec.BinaryCodec, storeService store.KVStoreService, authority string, addressCodec address.Codec) Keeper {
	auth, err := addressCodec.StringToBytes(authority)
	if err != nil {
		panic(err)
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		cdc:          cdc,
		storeService: storeService,
		authority:    auth,
		addressCodec: addressCodec,
		Permissions: collections.NewMap(
			sb,
			types.AccountPermissionPrefix,
			"permissions",
			collections.BytesKey,
			codec.CollValue[types.Permissions](cdc),
		),
		DisableList: collections.NewKeySet(
			sb,
			types.DisableListPrefix,
			"disable_list",
			collections.StringKey,
		),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

func (k *Keeper) GetAuthority() []byte {
	return k.authority
}

// IsAllowed returns true when msg URL is not found in the DisableList for given context, else false.
func (k *Keeper) IsAllowed(ctx context.Context, msgURL string) (bool, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(sdkCtx).FuncTiming(&sdkCtx, "IsAllowed")()

	has, err := k.DisableList.Has(sdkCtx, msgURL)
	return !has, err
}

func (k *Keeper) Meter(ctx context.Context) metrics.Meter {
	if k.meter == nil {
		k.meter = sdk.UnwrapSDKContext(ctx).Meter().SubMeter(types.ModuleName, metrics.Tag("svc", types.ModuleName))
	}

	return k.meter
}
