package main

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/bcp-innovations/hyperlane-cosmos/util"
	"github.com/bcp-innovations/hyperlane-cosmos/x/mailbox/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
	"log"
)

func init() {
	announceCmd.Flags().StringVarP(&privateKey, "private-key", "k", "", "Private key to sign the announcement")
	if err := announceCmd.MarkFlagRequired("private-key"); err != nil {
		panic(fmt.Errorf("failed to mark 'private-key' flag as required: %w", err))
	}

	announceCmd.Flags().StringVarP(&storageLocation, "storage-location", "s", "", "Storage location")
	if err := announceCmd.MarkFlagRequired("storage-location"); err != nil {
		panic(fmt.Errorf("failed to mark 'storage-location' flag as required: %w", err))
	}

	announceCmd.Flags().StringVarP(&mailboxID, "mailbox-id", "m", "", "Mailbox ID")
	if err := announceCmd.MarkFlagRequired("mailbox-id"); err != nil {
		panic(fmt.Errorf("failed to mark 'mailbox-id' flag as required: %w", err))
	}

	announceCmd.Flags().Uint32VarP(&localDomain, "local-domain", "d", 0, "Local domain ID")
	if err := announceCmd.MarkFlagRequired("local-domain"); err != nil {
		panic(fmt.Errorf("failed to mark 'local-domain' flag as required: %w", err))
	}
}

var announceCmd = &cobra.Command{
	Use:   "announce",
	Short: "Signs a validator announcement digest",
	RunE: func(cmd *cobra.Command, args []string) error {
		signature, err := announce(privateKey, storageLocation, mailboxID, localDomain)
		if err != nil {
			return err
		}
		fmt.Println(signature)
		return nil
	},
}

func announce(privKey, storageLocation, mailbox string, localDomain uint32) (string, error) {
	mailboxId, err := util.DecodeHexAddress(mailbox)
	if err != nil {
		return "", err
	}

	announcementDigest := types.GetAnnouncementDigest(storageLocation, localDomain, mailboxId.Bytes())

	privateKey, err := crypto.HexToECDSA(privKey)
	if err != nil {
		return "", err
	}

	publicKey := privateKey.Public()
	_, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	signedAnnouncement, err := crypto.Sign(announcementDigest, privateKey)
	if err != nil {
		log.Fatal(err)
	}

	return util.EncodeEthHex(signedAnnouncement), nil
}
