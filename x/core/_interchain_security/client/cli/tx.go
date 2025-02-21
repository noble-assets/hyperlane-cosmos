package cli

import (
	"strconv"
	"strings"

	"github.com/bcp-innovations/hyperlane-cosmos/x/core/_interchain_security/types"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"
)

func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        "ism-refactored", // TODO change command to "ism" once migrated
		Short:                      "Hyperlane Interchain Security Module commands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		CmdCreateMultiSigIsm(),
		CmdCreateNoopIsm(),
	)

	return txCmd
}

func CmdCreateMultiSigIsm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-merkle-root-multi-sig-ism [validators] [threshold]",
		Short: "Create a Hyperlane MultiSig ISM",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			validators := strings.Split(args[0], ",")
			threshold, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.MsgCreateMerkleRootMultiSigIsm{
				Creator:    clientCtx.GetFromAddress().String(),
				Validators: validators,
				Threshold:  uint32(threshold),
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func CmdCreateNoopIsm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-noop-ism",
		Short: "Create a Hyperlane Noop ISM",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.MsgCreateNoopIsm{
				Creator: clientCtx.GetFromAddress().String(),
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
