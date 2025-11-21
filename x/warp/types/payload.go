package types

import (
	"bytes"
	"errors"
	"math/big"
	"slices"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Slice position and length of the Warp payload components.
const (
	RecipientIndex = 0
	RecipientLen   = 32
	AmountIndex    = RecipientIndex + RecipientLen
	AmountLen      = 32
	PayloadIndex   = AmountIndex + AmountLen
)

const (
	// AddressLen is the length of an address in bytes.
	AddressLen = 20
	// PaddedLen represents the padded len of a 32 bytes array when used
	// to represent a cross-chain address.
	PaddedLen = RecipientLen - AddressLen
)

type WarpPayload struct {
	recipient []byte
	amount    big.Int
}

func NewWarpPayload(recipient []byte, amount big.Int) (WarpPayload, error) {
	if len(amount.Bytes()) > AmountLen {
		return WarpPayload{}, errors.New("amount is too long")
	}
	if len(recipient) > RecipientLen {
		return WarpPayload{}, errors.New("recipient address is too long")
	}

	return WarpPayload{recipient: recipient, amount: amount}, nil
}

func ParseWarpPayload(payload []byte) (WarpPayload, error) {
	if len(payload) < PayloadIndex {
		return WarpPayload{}, errors.New("payload is invalid")
	}

	amount := big.NewInt(0).SetBytes(payload[AmountIndex : AmountIndex+AmountLen])

	return WarpPayload{
		recipient: payload[RecipientIndex : RecipientIndex+RecipientLen],
		amount:    *amount,
	}, nil
}

func isZeroPadded(bz []byte) bool {
	return bytes.HasPrefix(bz, make([]byte, PaddedLen))
}

func (p WarpPayload) GetCosmosAccount() sdk.AccAddress {
	// If address is zero padded it is a 20-byte default cosmos address
	if isZeroPadded(p.recipient) {
		return p.recipient[PaddedLen:RecipientLen]
	}
	// if the address is not zero-padded, it might be a 32-byte address
	return p.recipient
}

func (p WarpPayload) Recipient() []byte {
	return p.recipient
}

func (p WarpPayload) Amount() *big.Int {
	newInt := big.NewInt(0)
	newInt.Set(&p.amount)
	return newInt
}

func (p WarpPayload) Bytes() []byte {
	intBytes := p.amount.Bytes()
	amountBytes := make([]byte, AmountLen)
	copy(amountBytes[AmountLen-len(intBytes):], intBytes)

	recBytes := p.recipient
	receiverBytes := make([]byte, RecipientLen)
	copy(receiverBytes[RecipientLen-len(recBytes):], recBytes)

	return slices.Concat(
		receiverBytes,
		amountBytes,
	)
}
