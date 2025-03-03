package keeper

import (
	"context"
	"fmt"

	"github.com/bcp-innovations/hyperlane-cosmos/x/core/02_post_dispatch/types"

	"cosmossdk.io/collections"
	"github.com/bcp-innovations/hyperlane-cosmos/util"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) Claim(ctx context.Context, sender string, igpId util.HexAddress) error {
	igp, err := k.Igps.Get(ctx, igpId.GetInternalId())
	if err != nil {
		return fmt.Errorf("failed to find ism with id: %s", igpId.String())
	}

	if sender != igp.Owner {
		return fmt.Errorf("failed to claim: %s is not permitted to claim", sender)
	}

	if igp.ClaimableFees.IsZero() {
		return fmt.Errorf("no claimable fees left")
	}

	ownerAcc, err := sdk.AccAddressFromBech32(igp.Owner)
	if err != nil {
		return err
	}

	// TODO use core-types module name or create sub-account
	err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, "hyperlane", ownerAcc, igp.ClaimableFees)
	if err != nil {
		return err
	}

	igp.ClaimableFees = sdk.NewCoins()

	err = k.Igps.Set(ctx, igpId.GetInternalId(), igp)
	if err != nil {
		return err
	}

	return nil
}

func (k Keeper) SetDestinationGasConfig(ctx context.Context, igpId util.HexAddress, owner string, destinationGasConfig *types.DestinationGasConfig) error {
	igp, err := k.Igps.Get(ctx, igpId.GetInternalId())
	if err != nil {
		return fmt.Errorf("igp does not exist: %s", igpId.String())
	}

	if igp.Owner != owner {
		return fmt.Errorf("failed to set DestinationGasConfigs: %s is not the owner of igp with id %s", owner, igpId.String())
	}

	if destinationGasConfig.GasOracle == nil {
		return fmt.Errorf("failed to set DestinationGasConfigs: gas Oracle is required")
	}

	updatedDestinationGasConfig := types.DestinationGasConfig{
		RemoteDomain: destinationGasConfig.RemoteDomain,
		GasOracle:    destinationGasConfig.GasOracle,
		GasOverhead:  destinationGasConfig.GasOverhead,
	}

	key := collections.Join(igpId.GetInternalId(), destinationGasConfig.RemoteDomain)

	if err = k.IgpDestinationGasConfigs.Set(ctx, key, updatedDestinationGasConfig); err != nil {
		return err
	}
	return nil
}
