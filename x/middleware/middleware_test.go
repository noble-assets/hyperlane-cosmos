package middleware_test

import (
	"context"
	"errors"

	"cosmossdk.io/math"

	"math/big"

	i "github.com/bcp-innovations/hyperlane-cosmos/tests/integration"
	"github.com/bcp-innovations/hyperlane-cosmos/util"
	coreTypes "github.com/bcp-innovations/hyperlane-cosmos/x/core/types"
	"github.com/bcp-innovations/hyperlane-cosmos/x/middleware"
	"github.com/bcp-innovations/hyperlane-cosmos/x/warp/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type MockKeeper struct {
	IsFailingPre  bool
	IsFailingPost bool
	NumPreCall    int
	NumPostCall   int
}

func (k *MockKeeper) HandlePre(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error {
	if k.IsFailingPre {
		return errors.New("failing in pre handle")
	}

	return nil
}

func (k *MockKeeper) HandlePost(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error {
	if k.IsFailingPost {
		return errors.New("failing in post handle")
	}

	return nil
}

var _ = Describe("middleware.go", Ordered, func() {
	var s *i.KeeperTestSuite
	var owner i.TestValidatorAddress
	var sender i.TestValidatorAddress
	var denom = "acoin"
	var keeper MockKeeper

	BeforeEach(func() {
		keeper = MockKeeper{}
		hook := middleware.NewHandleHook(
			middleware.WithPreHandleFn(keeper.HandlePre),
			middleware.WithPostHandleFn(keeper.HandlePost),
		)

		withHook := func(tokenType types.HypTokenType, hook middleware.HandleHook) func(*i.KeeperTestSuite) {
			return func(s *i.KeeperTestSuite) {
				s.HandleHooks[tokenType] = hook
			}
		}
		s = i.NewCleanChain(withHook(types.HYP_TOKEN_TYPE_COLLATERAL, hook))
		owner = i.GenerateTestValidatorAddress("Owner")
		sender = i.GenerateTestValidatorAddress("Sender")
		err := s.MintBaseCoins(owner.Address, 1_000_000)
		Expect(err).To(BeNil())
	})

	Context("wrapping the collateral Warp application", func() {
		When("the middleware returns an error", func() {
			var (
				message   util.HyperlaneMessage
				mailboxId util.HexAddress
			)

			BeforeEach(func() {
				keeper.IsFailingPre = false
				keeper.IsFailingPost = false
				// Arrange
				receiverAddress, _ := util.DecodeHexAddress("0xd7194459d45619d04a5a0f9e78dc9594a0f37fd6da8382fe12ddda6f2f46d647")
				remoteRouter := types.RemoteRouter{
					ReceiverDomain:   1,
					ReceiverContract: "0x934b867052ca9c65e33362112f35fb548f8732c2fe45f07b9c591958e865def0",
					Gas:              math.NewInt(50000),
				}

				amount := math.NewInt(100)
				maxFee := sdk.NewCoin(denom, math.NewInt(250000))

				var tokenId, igpId util.HexAddress
				tokenId, mailboxId, igpId, _ = i.CreateToken(s, &remoteRouter, owner.Address, sender.Address, types.HYP_TOKEN_TYPE_COLLATERAL)
				err := s.MintBaseCoins(sender.Address, 1_000_000)
				Expect(err).To(BeNil())

				senderBalance := s.App().BankKeeper.GetBalance(s.Ctx(), sender.AccAddress, denom)
				// Act
				_, err = s.RunTx(&types.MsgRemoteTransfer{
					Sender:            sender.Address,
					TokenId:           tokenId,
					DestinationDomain: remoteRouter.ReceiverDomain,
					Recipient:         receiverAddress,
					Amount:            amount,
					CustomHookId:      &igpId,
					GasLimit:          math.ZeroInt(),
					MaxFee:            maxFee,
				})
				Expect(err).To(BeNil())

				Expect(s.App().BankKeeper.GetBalance(s.Ctx(), sender.AccAddress, denom).Amount).To(Equal(senderBalance.Amount.Sub(amount.Add(maxFee.Amount))))

				receiverContract, err := util.DecodeHexAddress(remoteRouter.ReceiverContract)
				Expect(err).To(BeNil())

				warpRecipient, err := sdk.GetFromBech32(sender.Address, "hyp")
				Expect(err).To(BeNil())

				warpPayload, err := types.NewWarpPayload(warpRecipient, *big.NewInt(amount.Int64()))
				Expect(err).To(BeNil())

				message = util.HyperlaneMessage{
					Version:     3,
					Nonce:       1,
					Origin:      remoteRouter.ReceiverDomain,
					Sender:      receiverContract,
					Destination: 0,
					Recipient:   tokenId,
					Body:        warpPayload.Bytes(),
				}

				senderBalance = s.App().BankKeeper.GetBalance(s.Ctx(), sender.AccAddress, denom)

			})

			It("MsgProcessMessage fails (pre handle hook)", func() {
				keeper.IsFailingPre = true

				_, err := s.RunTx(&coreTypes.MsgProcessMessage{
					MailboxId: mailboxId,
					Relayer:   sender.Address,
					Metadata:  "",
					Message:   message.String(),
				})
				Expect(err).To(MatchError(ContainSubstring("pre-handle hook failed")))
			})

			It("MsgProcessMessage fails (post handle hook)", func() {
				keeper.IsFailingPost = true

				_, err := s.RunTx(&coreTypes.MsgProcessMessage{
					MailboxId: mailboxId,
					Relayer:   sender.Address,
					Metadata:  "",
					Message:   message.String(),
				})
				Expect(err).To(MatchError(ContainSubstring("post-handle hook failed")))
			})
		})

		When("the middleware does NOT return an error", func() {
			keeper.IsFailingPre = false
			keeper.IsFailingPost = false

			It("MsgProcessMessage succeeds", func() {
				receiverAddress, _ := util.DecodeHexAddress("0xd7194459d45619d04a5a0f9e78dc9594a0f37fd6da8382fe12ddda6f2f46d647")
				remoteRouter := types.RemoteRouter{
					ReceiverDomain:   1,
					ReceiverContract: "0x934b867052ca9c65e33362112f35fb548f8732c2fe45f07b9c591958e865def0",
					Gas:              math.NewInt(50000),
				}

				amount := math.NewInt(100)
				maxFee := sdk.NewCoin(denom, math.NewInt(250000))

				tokenId, mailboxId, igpId, _ := i.CreateToken(s, &remoteRouter, owner.Address, sender.Address, types.HYP_TOKEN_TYPE_COLLATERAL)
				err := s.MintBaseCoins(sender.Address, 1_000_000)
				Expect(err).To(BeNil())

				senderBalance := s.App().BankKeeper.GetBalance(s.Ctx(), sender.AccAddress, denom)
				// Act
				_, err = s.RunTx(&types.MsgRemoteTransfer{
					Sender:            sender.Address,
					TokenId:           tokenId,
					DestinationDomain: remoteRouter.ReceiverDomain,
					Recipient:         receiverAddress,
					Amount:            amount,
					CustomHookId:      &igpId,
					GasLimit:          math.ZeroInt(),
					MaxFee:            maxFee,
				})
				Expect(err).To(BeNil())

				Expect(s.App().BankKeeper.GetBalance(s.Ctx(), sender.AccAddress, denom).Amount).To(Equal(senderBalance.Amount.Sub(amount.Add(maxFee.Amount))))

				receiverContract, err := util.DecodeHexAddress(remoteRouter.ReceiverContract)
				Expect(err).To(BeNil())

				warpRecipient, err := sdk.GetFromBech32(sender.Address, "hyp")
				Expect(err).To(BeNil())

				warpPayload, err := types.NewWarpPayload(warpRecipient, *big.NewInt(amount.Int64()))
				Expect(err).To(BeNil())

				message := util.HyperlaneMessage{
					Version:     3,
					Nonce:       1,
					Origin:      remoteRouter.ReceiverDomain,
					Sender:      receiverContract,
					Destination: 0,
					Recipient:   tokenId,
					Body:        warpPayload.Bytes(),
				}

				senderBalance = s.App().BankKeeper.GetBalance(s.Ctx(), sender.AccAddress, denom)

				_, err = s.RunTx(&coreTypes.MsgProcessMessage{
					MailboxId: mailboxId,
					Relayer:   sender.Address,
					Metadata:  "",
					Message:   message.String(),
				})

				Expect(err).To(BeNil())
				Expect(s.App().BankKeeper.GetBalance(s.Ctx(), sender.AccAddress, denom).Amount).To(Equal(senderBalance.Amount.Add(amount)))

				Expect(keeper.NumPreCall).To(Equal(1))
				Expect(keeper.NumPostCall).To(Equal(1))
			})
		})
	})

})
