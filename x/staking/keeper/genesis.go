package keeper

import (
	"context"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// InitGenesis sets the pool and parameters for the provided keeper.  For each
// validator in data, it sets that validator in the keeper along with manually
// setting the indexes. In addition, it also sets any delegations found in
// data. Finally, it updates the bonded validators.
// Returns final validator set after applying all declaration and delegations
func (k Keeper) InitGenesis(ctx context.Context, data *types.GenesisState) (res []abci.ValidatorUpdate) {
	var err error
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	defer k.Meter(ctx).FuncTiming(&sdkCtx, "InitGenesis")(&err)

	bondedTokens := math.ZeroInt()
	notBondedTokens := math.ZeroInt()

	// We need to pretend to be "n blocks before genesis", where "n" is the
	// validator update delay, so that e.g. slashing periods are correctly
	// initialized for the validator set e.g. with a one-block offset - the
	// first TM block is at height 1, so state updates applied from
	// genesis.json are in block 0.
	sdkCtx = sdkCtx.WithBlockHeight(1 - sdk.ValidatorUpdateDelay)

	if err = k.SetParams(sdkCtx, data.Params); err != nil {
		panic(err)
	}

	if err = k.SetLastTotalPower(sdkCtx, data.LastTotalPower); err != nil {
		panic(err)
	}

	for _, validator := range data.Validators {
		if err = k.SetValidator(sdkCtx, validator); err != nil {
			panic(err)
		}

		// Manually set indices for the first time
		if err = k.SetValidatorByConsAddr(sdkCtx, validator); err != nil {
			panic(err)
		}

		if err = k.SetValidatorByPowerIndex(sdkCtx, validator); err != nil {
			panic(err)
		}

		// Call the creation hook if not exported
		if !data.Exported {
			valbz, err := k.ValidatorAddressCodec().StringToBytes(validator.GetOperator())
			if err != nil {
				panic(err)
			}
			if err = k.Hooks().AfterValidatorCreated(sdkCtx, valbz); err != nil {
				panic(err)
			}
		}

		// update timeslice if necessary
		if validator.IsUnbonding() {
			if err = k.InsertUnbondingValidatorQueue(sdkCtx, validator); err != nil {
				panic(err)
			}
		}

		switch validator.GetStatus() {
		case types.Bonded:
			bondedTokens = bondedTokens.Add(validator.GetTokens())

		case types.Unbonding, types.Unbonded:
			notBondedTokens = notBondedTokens.Add(validator.GetTokens())

		default:
			err = fmt.Errorf("invalid validator status")
			panic(err)
		}
	}

	for _, delegation := range data.Delegations {
		delegatorAddress, err := k.authKeeper.AddressCodec().StringToBytes(delegation.DelegatorAddress)
		if err != nil {
			err = fmt.Errorf("invalid delegator address: %s", err)
			panic(err)
		}

		valAddr, err := k.validatorAddressCodec.StringToBytes(delegation.GetValidatorAddr())
		if err != nil {
			panic(err)
		}

		// Call the before-creation hook if not exported
		if !data.Exported {
			if err = k.Hooks().BeforeDelegationCreated(sdkCtx, delegatorAddress, valAddr); err != nil {
				panic(err)
			}
		}

		if err = k.SetDelegation(sdkCtx, delegation); err != nil {
			panic(err)
		}

		// Call the after-modification hook if not exported
		if !data.Exported {
			if err = k.Hooks().AfterDelegationModified(sdkCtx, delegatorAddress, valAddr); err != nil {
				panic(err)
			}
		}
	}

	for _, ubd := range data.UnbondingDelegations {
		if err = k.SetUnbondingDelegation(sdkCtx, ubd); err != nil {
			panic(err)
		}

		for _, entry := range ubd.Entries {
			if err = k.InsertUBDQueue(sdkCtx, ubd, entry.CompletionTime); err != nil {
				panic(err)
			}
			notBondedTokens = notBondedTokens.Add(entry.Balance)
		}
	}

	for _, red := range data.Redelegations {
		if err = k.SetRedelegation(sdkCtx, red); err != nil {
			panic(err)
		}

		for _, entry := range red.Entries {
			if err = k.InsertRedelegationQueue(sdkCtx, red, entry.CompletionTime); err != nil {
				panic(err)
			}
		}
	}

	bondedCoins := sdk.NewCoins(sdk.NewCoin(data.Params.BondDenom, bondedTokens))
	notBondedCoins := sdk.NewCoins(sdk.NewCoin(data.Params.BondDenom, notBondedTokens))

	// check if the unbonded and bonded pools accounts exists
	bondedPool := k.GetBondedPool(sdkCtx)
	if bondedPool == nil {
		err = fmt.Errorf("%s module account has not been set", types.BondedPoolName)
		panic(err)
	}

	// TODO: remove with genesis 2-phases refactor https://github.com/cosmos/cosmos-sdk/issues/2862

	bondedBalance := k.bankKeeper.GetAllBalances(sdkCtx, bondedPool.GetAddress())
	if bondedBalance.IsZero() {
		k.authKeeper.SetModuleAccount(sdkCtx, bondedPool)
	}

	// if balance is different from bonded coins panic because genesis is most likely malformed
	if !bondedBalance.Equal(bondedCoins) {
		err = fmt.Errorf("bonded pool balance is different from bonded coins: %s <-> %s", bondedBalance, bondedCoins)
		panic(err)
	}

	notBondedPool := k.GetNotBondedPool(sdkCtx)
	if notBondedPool == nil {
		err = fmt.Errorf("%s module account has not been set", types.NotBondedPoolName)
		panic(err)
	}

	notBondedBalance := k.bankKeeper.GetAllBalances(sdkCtx, notBondedPool.GetAddress())
	if notBondedBalance.IsZero() {
		k.authKeeper.SetModuleAccount(sdkCtx, notBondedPool)
	}

	// If balance is different from non bonded coins panic because genesis is most
	// likely malformed.
	if !notBondedBalance.Equal(notBondedCoins) {
		err = fmt.Errorf("not bonded pool balance is different from not bonded coins: %s <-> %s", notBondedBalance, notBondedCoins)
		panic(err)
	}

	// don't need to run CometBFT updates if we exported
	if data.Exported {
		for _, lv := range data.LastValidatorPowers {
			valAddr, err := k.validatorAddressCodec.StringToBytes(lv.Address)
			if err != nil {
				panic(err)
			}

			err = k.SetLastValidatorPower(sdkCtx, valAddr, lv.Power)
			if err != nil {
				panic(err)
			}

			validator, err := k.GetValidator(sdkCtx, valAddr)
			if err != nil {
				err = fmt.Errorf("validator %s not found", lv.Address)
				panic(err)
			}

			update := validator.ABCIValidatorUpdate(k.PowerReduction(sdkCtx))
			update.Power = lv.Power // keep the next-val-set offset, use the last power for the first block
			res = append(res, update)
		}
	} else {
		res, err = k.ApplyAndReturnValidatorSetUpdates(sdkCtx)
		if err != nil {
			panic(err)
		}
	}

	return res
}

// ExportGenesis returns a GenesisState for a given context and keeper. The
// GenesisState will contain the pool, params, validators, and bonds found in
// the keeper.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	var err error
	defer k.Meter(ctx).FuncTiming(&ctx, "ExportGenesis")(&err)

	var unbondingDelegations []types.UnbondingDelegation

	err = k.IterateUnbondingDelegations(ctx, func(_ int64, ubd types.UnbondingDelegation) (stop bool) {
		unbondingDelegations = append(unbondingDelegations, ubd)
		return false
	})
	if err != nil {
		panic(err)
	}

	var redelegations []types.Redelegation

	err = k.IterateRedelegations(ctx, func(_ int64, red types.Redelegation) (stop bool) {
		redelegations = append(redelegations, red)
		return false
	})
	if err != nil {
		panic(err)
	}

	var lastValidatorPowers []types.LastValidatorPower

	err = k.IterateLastValidatorPowers(ctx, func(addr sdk.ValAddress, power int64) (stop bool) {
		addrStr, err := k.validatorAddressCodec.BytesToString(addr)
		if err != nil {
			panic(err)
		}
		lastValidatorPowers = append(lastValidatorPowers, types.LastValidatorPower{Address: addrStr, Power: power})
		return false
	})
	if err != nil {
		panic(err)
	}

	params, err := k.GetParams(ctx)
	if err != nil {
		panic(err)
	}

	totalPower, err := k.GetLastTotalPower(ctx)
	if err != nil {
		panic(err)
	}

	allDelegations, err := k.GetAllDelegations(ctx)
	if err != nil {
		panic(err)
	}

	allValidators, err := k.GetAllValidators(ctx)
	if err != nil {
		panic(err)
	}

	return &types.GenesisState{
		Params:               params,
		LastTotalPower:       totalPower,
		LastValidatorPowers:  lastValidatorPowers,
		Validators:           allValidators,
		Delegations:          allDelegations,
		UnbondingDelegations: unbondingDelegations,
		Redelegations:        redelegations,
		Exported:             true,
	}
}
