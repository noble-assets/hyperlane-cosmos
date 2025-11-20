package middleware

import (
	"context"

	"github.com/bcp-innovations/hyperlane-cosmos/util"
)

// HandleFn is the signature of the Handle function of an Hyperlane application.
type HandleFn = func(ctx context.Context, mailboxId util.HexAddress, message util.HyperlaneMessage) error

type HandleHook interface {
	PreHandle(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error
	PostHandle(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error
}

var _ HandleHook = &Hook{}

type Hook struct {
	// preHandleHook is the behavior expected from a type that hook BEFORE executing
	// the Hyperlane application Handle method.
	preHandleFn HandleFn

	// postHandleHook is the behavior expected from a type that hook AFTER executing
	// the Hyperlane application Handle method.
	postHandleFn HandleFn
}

func (h *Hook) PreHandle(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error {
	if h.preHandleFn != nil {
		return h.preHandleFn(ctx, mailboxID, message)
	}

	return nil
}

func (h *Hook) PostHandle(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error {
	if h.postHandleFn != nil {
		return h.postHandleFn(ctx, mailboxID, message)
	}

	return nil
}

func NewHook(opts ...HookOpt) *Hook {
	h := &Hook{}

	for _, opt := range opts {
		opt(h)
	}
	return h
}

type HookOpt func(*Hook)

func WithPreHandleFn(fn HandleFn) HookOpt {
	return func(h *Hook) {
		h.preHandleFn = fn
	}
}

func WithPostHandleFn(fn HandleFn) HookOpt {
	return func(h *Hook) {
		h.postHandleFn = fn
	}
}
