package integration

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"

	. "github.com/onsi/gomega"

	"github.com/bcp-innovations/hyperlane-cosmos/tests/simapp"
	"github.com/bcp-innovations/hyperlane-cosmos/util"
	ismTypes "github.com/bcp-innovations/hyperlane-cosmos/x/core/01_interchain_security/types"
	pdTypes "github.com/bcp-innovations/hyperlane-cosmos/x/core/02_post_dispatch/types"
	coreKeeper "github.com/bcp-innovations/hyperlane-cosmos/x/core/keeper"
	coreTypes "github.com/bcp-innovations/hyperlane-cosmos/x/core/types"
	"github.com/bcp-innovations/hyperlane-cosmos/x/middleware"
	"github.com/bcp-innovations/hyperlane-cosmos/x/warp"
	"github.com/bcp-innovations/hyperlane-cosmos/x/warp/keeper"
	"github.com/bcp-innovations/hyperlane-cosmos/x/warp/types"
)

var denom = "acoin"

// postBuildOpts returns a slice of post application build options.
func (suite *KeeperTestSuite) postBuildOpts() simapp.PostBuildOpts {
	return []simapp.PostBuildOpt{
		suite.setupWarpApps(),
	}
}

// setupWarpApps returns a post application build configuration handler to
// register the Warp applications on the Hyperlane core. If middleware hooks
// have been specified in the suite, they are used to wrap the warp keeper.
func (suite *KeeperTestSuite) setupWarpApps() simapp.PostBuildOpt {
	// If no custom hooks, use default registration
	if len(suite.HandleHooks) == 0 {
		return simapp.RegisterDefaultWarpAppsOpt()
	}

	return func(app *simapp.App) {
		defaultWarpApps := warp.DefaultWarpApps(&app.WarpKeeper)

		warpAppsMap := make(map[types.HypTokenType]util.HyperlaneApp, len(defaultWarpApps))
		for _, defaultWarpApp := range defaultWarpApps {
			warpAppsMap[defaultWarpApp.TokenType] = defaultWarpApp.Handler
		}

		// Wrap app with custom middleware if hooks are provided.
		for tokenType, hook := range suite.HandleHooks {
			app, ok := warpAppsMap[tokenType]
			Expect(
				ok,
			).To(BeTrue(), fmt.Sprintf("expected warp app for token type %s to be found", tokenType))

			appMiddleware, err := middleware.New(app, hook)
			Expect(err).To(BeNil())

			warpAppsMap[tokenType] = appMiddleware
		}

		warpApps := make([]warp.WarpApp, 0, len(warpAppsMap))
		for tokenType, warpApp := range warpAppsMap {
			warpApps = append(warpApps, warp.WarpApp{
				TokenType: tokenType,
				Handler:   warpApp,
			})
		}

		app.RegisterWarpApps(warpApps...)
	}
}

func createIgp(s *KeeperTestSuite, creator string) util.HexAddress {
	res, err := s.RunTx(&pdTypes.MsgCreateIgp{
		Owner: creator,
		Denom: denom,
	})
	Expect(err).To(BeNil())

	var response pdTypes.MsgCreateIgpResponse
	err = proto.Unmarshal(res.MsgResponses[0].Value, &response)
	Expect(err).To(BeNil())

	return response.Id
}

func createMerkleHook(
	s *KeeperTestSuite,
	creator string,
	mailboxId util.HexAddress,
) util.HexAddress {
	res, err := s.RunTx(&pdTypes.MsgCreateMerkleTreeHook{
		Owner:     creator,
		MailboxId: mailboxId,
	})
	Expect(err).To(BeNil())

	var response pdTypes.MsgCreateMerkleTreeHookResponse
	err = proto.Unmarshal(res.MsgResponses[0].Value, &response)
	Expect(err).To(BeNil())

	return response.Id
}

func CreateValidMailbox(
	s *KeeperTestSuite,
	creator string,
	ism string,
	destinationDomain uint32,
) (util.HexAddress, util.HexAddress, util.HexAddress) {
	var ismId util.HexAddress
	switch ism {
	case "noop":
		ismId = CreateNoopIsm(s, creator)
	case "multisig":
		ismId = createMultisigIsm(s, creator)
	}

	igpId := createIgp(s, creator)

	err := setDestinationGasConfig(s, creator, igpId, destinationDomain)
	Expect(err).To(BeNil())

	res, err := s.RunTx(&coreTypes.MsgCreateMailbox{
		Owner:      creator,
		DefaultIsm: ismId,
	})
	Expect(err).To(BeNil())

	var response coreTypes.MsgCreateMailboxResponse
	err = proto.Unmarshal(res.MsgResponses[0].Value, &response)
	Expect(err).To(BeNil())
	mailboxId := response.Id

	merkleHook := createMerkleHook(s, creator, mailboxId)

	_, err = s.RunTx(&coreTypes.MsgSetMailbox{
		Owner:        creator,
		MailboxId:    mailboxId,
		DefaultIsm:   &ismId,
		DefaultHook:  &igpId,
		RequiredHook: &merkleHook,
		NewOwner:     creator,
	})
	Expect(err).To(BeNil())

	if err != nil {
		return [32]byte{}, [32]byte{}, [32]byte{}
	}

	return verifyNewMailbox(s, res, creator, igpId.String(), ismId.String()), igpId, ismId
}

func createMultisigIsm(s *KeeperTestSuite, creator string) util.HexAddress {
	res, err := s.RunTx(&ismTypes.MsgCreateMerkleRootMultisigIsm{
		Creator: creator,
		Validators: []string{
			"0xb05b6a0aa112b61a7aa16c19cac27d970692995e",
			"0xa05b6a0aa112b61a7aa16c19cac27d970692995e",
			"0xd05b6a0aa112b61a7aa16c19cac27d970692995e",
		},
		Threshold: 2,
	})
	Expect(err).To(BeNil())

	var response ismTypes.MsgCreateMerkleRootMultisigIsmResponse
	err = proto.Unmarshal(res.MsgResponses[0].Value, &response)
	Expect(err).To(BeNil())

	return response.Id
}

func CreateNoopIsm(s *KeeperTestSuite, creator string) util.HexAddress {
	res, err := s.RunTx(&ismTypes.MsgCreateNoopIsm{
		Creator: creator,
	})
	Expect(err).To(BeNil())

	var response ismTypes.MsgCreateNoopIsmResponse
	err = proto.Unmarshal(res.MsgResponses[0].Value, &response)
	Expect(err).To(BeNil())

	return response.Id
}

func setDestinationGasConfig(
	s *KeeperTestSuite,
	creator string,
	igpId util.HexAddress,
	domain uint32,
) error {
	_, err := s.RunTx(&pdTypes.MsgSetDestinationGasConfig{
		Owner: creator,
		IgpId: igpId,
		DestinationGasConfig: &pdTypes.DestinationGasConfig{
			RemoteDomain: domain,
			GasOracle: &pdTypes.GasOracle{
				TokenExchangeRate: math.NewInt(1e10),
				GasPrice:          math.NewInt(1),
			},
			GasOverhead: math.NewInt(200000),
		},
	})

	return err
}

func verifyNewMailbox(
	s *KeeperTestSuite,
	res *sdk.Result,
	creator, igpId, ismId string,
) util.HexAddress {
	var response coreTypes.MsgCreateMailboxResponse
	err := proto.Unmarshal(res.MsgResponses[0].Value, &response)
	Expect(err).To(BeNil())
	mailboxId := response.Id

	mailbox, err := s.App().HyperlaneKeeper.Mailboxes.Get(s.Ctx(), mailboxId.GetInternalId())
	Expect(err).To(BeNil())
	Expect(mailbox.Owner).To(Equal(creator))
	Expect(mailbox.DefaultIsm.String()).To(Equal(ismId))
	Expect(mailbox.MessageSent).To(Equal(uint32(0)))
	Expect(mailbox.MessageReceived).To(Equal(uint32(0)))
	if igpId != "" {
		Expect(mailbox.DefaultHook.String()).To(Equal(igpId))
	} else {
		Expect(mailbox.DefaultHook).To(BeNil())
	}

	mailboxes, err := coreKeeper.NewQueryServerImpl(s.App().HyperlaneKeeper).
		Mailboxes(s.Ctx(), &coreTypes.QueryMailboxesRequest{})
	Expect(err).To(BeNil())
	Expect(mailboxes.Mailboxes).To(HaveLen(1))
	Expect(mailboxes.Mailboxes[0].Owner).To(Equal(creator))

	return mailboxId
}

func CreateToken(
	s *KeeperTestSuite,
	remoteRouter *types.RemoteRouter,
	owner, _ string,
	tokenType types.HypTokenType,
) (util.HexAddress, util.HexAddress, util.HexAddress, util.HexAddress) {
	mailboxId, igpId, ismId := CreateValidMailbox(s, owner, "noop", 1)

	var tokenId util.HexAddress
	switch tokenType {
	case 1:
		res, err := s.RunTx(&types.MsgCreateCollateralToken{
			Owner:         owner,
			OriginDenom:   denom,
			OriginMailbox: mailboxId,
		})
		Expect(err).To(BeNil())

		var response types.MsgCreateCollateralTokenResponse
		err = proto.Unmarshal(res.MsgResponses[0].Value, &response)
		Expect(err).To(BeNil())
		tokenId = response.Id

	case 2:
		res, err := s.RunTx(&types.MsgCreateSyntheticToken{
			Owner:         owner,
			OriginMailbox: mailboxId,
		})
		Expect(err).To(BeNil())

		var response types.MsgCreateSyntheticTokenResponse
		err = proto.Unmarshal(res.MsgResponses[0].Value, &response)
		Expect(err).To(BeNil())
		tokenId = response.Id
	}

	if remoteRouter != nil {
		_, err := s.RunTx(&types.MsgEnrollRemoteRouter{
			Owner:        owner,
			TokenId:      tokenId,
			RemoteRouter: remoteRouter,
		})
		Expect(err).To(BeNil())
	}

	_, err := s.RunTx(&types.MsgSetToken{
		Owner:    owner,
		TokenId:  tokenId,
		IsmId:    &ismId,
		NewOwner: "",
	})
	Expect(err).To(BeNil())

	tokens, err := keeper.NewQueryServerImpl(s.App().WarpKeeper).
		Tokens(s.Ctx(), &types.QueryTokensRequest{})
	Expect(err).To(BeNil())
	Expect(tokens.Tokens).To(HaveLen(1))
	Expect(tokens.Tokens[0].Owner).To(Equal(owner))

	routers, err := keeper.NewQueryServerImpl(s.App().WarpKeeper).
		RemoteRouters(s.Ctx(), &types.QueryRemoteRoutersRequest{
			Id: tokenId.String(),
		})
	Expect(err).To(BeNil())

	if remoteRouter != nil {
		Expect(routers.RemoteRouters).To(HaveLen(1))
		Expect(routers.RemoteRouters[0]).To(Equal(remoteRouter))
	} else {
		Expect(routers.RemoteRouters).To(HaveLen(0))
	}
	return tokenId, mailboxId, igpId, ismId
}
