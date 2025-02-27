package keeper

import (
	"context"
	"fmt"
	"slices"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/core/store"

	"github.com/bcp-innovations/hyperlane-cosmos/util"
	"github.com/bcp-innovations/hyperlane-cosmos/x/warp/types"
	"github.com/cosmos/cosmos-sdk/codec"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	addressCodec address.Codec

	// authority is the address capable of executing a MsgUpdateParams and other authority-gated message.
	// typically, this should be the x/gov module account.
	authority string

	enabledTokens []int32

	// state management

	Params collections.Item[types.Params]
	Schema collections.Schema
	// <tokenId> -> Token
	HypTokens      collections.Map[[]byte, types.HypToken]
	HypTokensCount collections.Sequence
	// <tokenId> <domain> -> Router
	EnrolledRouters collections.Map[collections.Pair[[]byte, uint32], types.RemoteRouter]

	bankKeeper types.BankKeeper
	coreKeeper types.CoreKeeper
}

// NewKeeper creates a new Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	addressCodec address.Codec,
	storeService storetypes.KVStoreService,
	authority string,
	bankKeeper types.BankKeeper,
	coreKeeper types.CoreKeeper,
	enabledTokens []int32,
) Keeper {
	if _, err := addressCodec.StringToBytes(authority); err != nil {
		panic(fmt.Errorf("invalid authority address: %w", err))
	}

	sb := collections.NewSchemaBuilder(storeService)
	k := Keeper{
		cdc:             cdc,
		addressCodec:    addressCodec,
		authority:       authority,
		enabledTokens:   enabledTokens,
		HypTokens:       collections.NewMap(sb, types.HypTokenKey, "hyptokens", collections.BytesKey, codec.CollValue[types.HypToken](cdc)),
		Params:          collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		HypTokensCount:  collections.NewSequence(sb, types.HypTokensCountKey, "hyptokens_count"),
		EnrolledRouters: collections.NewMap(sb, types.EnrolledRoutersKey, "enrolled_routers", collections.PairKeyCodec(collections.BytesKey, collections.Uint32Key), codec.CollValue[types.RemoteRouter](cdc)),
		bankKeeper:      bankKeeper,
		coreKeeper:      coreKeeper,
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}

	coreKeeper.AppRouter().RegisterModule(uint8(types.HYP_TOKEN_TYPE_COLLATERAL), &k)
	coreKeeper.AppRouter().RegisterModule(uint8(types.HYP_TOKEN_TYPE_SYNTHETIC), &k)

	k.Schema = schema

	return k
}

func (k *Keeper) Exists(ctx context.Context, tokenId util.HexAddress) (bool, error) {
	return k.HypTokens.Has(ctx, tokenId.Bytes())
}

func (k *Keeper) ReceiverIsmId(ctx context.Context, recipient util.HexAddress) (util.HexAddress, error) {
	token, err := k.HypTokens.Get(ctx, recipient.Bytes())
	if err != nil {
		return util.HexAddress{}, nil
	}

	hexAddress, err := util.DecodeHexAddress(token.IsmId)
	if err != nil {
		return util.HexAddress{}, err
	}

	return hexAddress, nil
}

func (k *Keeper) Handle(ctx context.Context, mailboxId util.HexAddress, message util.HyperlaneMessage) error {
	token, err := k.HypTokens.Get(ctx, message.Recipient.Bytes())
	if err != nil {
		return nil
	}

	payload, err := types.ParseWarpPayload(message.Body)
	if err != nil {
		return err
	}

	if util.HexAddress(token.OriginMailbox) != mailboxId {
		return fmt.Errorf("invalid origin mailbox address")
	}

	remoteRouter, err := k.EnrolledRouters.Get(ctx, collections.Join(message.Recipient.Bytes(), message.Origin))
	if err != nil {
		return fmt.Errorf("no enrolled router found for origin %d", message.Origin)
	}

	if message.Sender.String() != remoteRouter.ReceiverContract {
		return fmt.Errorf("invalid receiver contract")
	}

	// Check token type
	err = nil
	if token.TokenType == types.HYP_TOKEN_TYPE_COLLATERAL {
		if !slices.Contains(k.enabledTokens, int32(types.HYP_TOKEN_TYPE_COLLATERAL)) {
			return fmt.Errorf("module disabled collateral tokens")
		}
		err = k.RemoteReceiveCollateral(ctx, token, payload)
	} else if token.TokenType == types.HYP_TOKEN_TYPE_SYNTHETIC {
		if !slices.Contains(k.enabledTokens, int32(types.HYP_TOKEN_TYPE_SYNTHETIC)) {
			return fmt.Errorf("module disabled synthetic tokens")
		}
		err = k.RemoteReceiveSynthetic(ctx, token, payload)
	} else {
		panic("inconsistent store")
	}

	return err
}
