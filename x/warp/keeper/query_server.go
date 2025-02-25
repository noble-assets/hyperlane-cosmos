package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"

	"github.com/bcp-innovations/hyperlane-cosmos/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/bcp-innovations/hyperlane-cosmos/x/warp/types"
)

var _ types.QueryServer = queryServer{}

// NewQueryServerImpl returns an implementation of the module QueryServer.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{k}
}

type queryServer struct {
	k Keeper
}

func (qs queryServer) RemoteRouters(ctx context.Context, request *types.QueryRemoteRoutersRequest) (*types.QueryRemoteRoutersResponse, error) {
	tokenId, err := util.DecodeHexAddress(request.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	rng := collections.NewPrefixedPairRange[uint64, uint32](tokenId.GetInternalId())

	// TODO: Add pagination
	iter, err := qs.k.EnrolledRouters.Iterate(ctx, rng)
	if err != nil {
		return &types.QueryRemoteRoutersResponse{}, err
	}

	routers, err := iter.Values()
	if err != nil {
		return &types.QueryRemoteRoutersResponse{}, err
	}

	remoteRouters := make([]*types.RemoteRouter, len(routers))
	for i := range routers {
		remoteRouters[i] = &routers[i]
	}

	return &types.QueryRemoteRoutersResponse{
		RemoteRouters: remoteRouters,
	}, nil
}

func (qs queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params, err := qs.k.Params.Get(ctx)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return &types.QueryParamsResponse{Params: types.Params{}}, nil
		}

		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryParamsResponse{Params: params}, nil
}

func (qs queryServer) Tokens(ctx context.Context, _ *types.QueryTokensRequest) (*types.QueryTokensResponse, error) {
	it, err := qs.k.HypTokens.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}

	tokens, err := it.Values()
	if err != nil {
		return nil, err
	}

	responseTokens := make([]types.QueryTokenResponse, len(tokens))
	for i, t := range tokens {
		responseTokens[i], err = qs.parseTokenResponse(ctx, t)
		if err != nil {
			return nil, err
		}
	}

	return &types.QueryTokensResponse{
		Tokens: responseTokens,
	}, nil
}

func (qs queryServer) Token(ctx context.Context, request *types.QueryTokenRequest) (*types.QueryTokenResponse, error) {
	tokenId, err := util.DecodeHexAddress(request.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	get, err := qs.k.HypTokens.Get(ctx, tokenId.GetInternalId())
	if err != nil {
		return nil, err
	}

	response, err := qs.parseTokenResponse(ctx, get)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (qs queryServer) parseTokenResponse(ctx context.Context, get types.HypToken) (types.QueryTokenResponse, error) {
	rng := collections.NewPrefixedPairRange[uint64, uint32](get.Id)

	iter, err := qs.k.EnrolledRouters.Iterate(ctx, rng)
	if err != nil {
		return types.QueryTokenResponse{}, err
	}

	routers, err := iter.Values()
	if err != nil {
		return types.QueryTokenResponse{}, err
	}

	remoteRouters := make([]*types.RemoteRouter, len(routers))
	for i := range routers {
		remoteRouters[i] = &routers[i]
	}

	return types.QueryTokenResponse{
		Id:        get.Id,
		Owner:     get.Owner,
		TokenType: get.TokenType,

		OriginMailbox: util.HexAddress(get.OriginMailbox).String(),
		OriginDenom:   get.OriginDenom,
		RemoteRouters: remoteRouters,

		IsmId: get.IsmId,
	}, nil
}
