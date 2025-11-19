package middleware

import (
	"context"
	"errors"
	"fmt"

	"github.com/bcp-innovations/hyperlane-cosmos/util"
)

// AppMiddleware defines the behavior required from an Hyperlane application middleware.
type AppMiddleware interface {
	util.HyperlaneApp
}

var _ AppMiddleware = (*Middleware)(nil)

// Middleware implements the expected Hyperlane application middleware.
type Middleware struct {
	util.HyperlaneApp
	hook HandleHook
}

func NewMiddleware(inner util.HyperlaneApp, hook HandleHook) (*Middleware, error) {
	if inner == nil {
		return nil, errors.New("inner Hyperlane application cannot be nil")
	}
	if hook == nil {
		return nil, errors.New("hook cannot be nil")
	}

	return &Middleware{
		HyperlaneApp: inner,
		hook:         hook,
	}, nil
}

// Exists dispatch the request to the underlying wrapped Hyperlane application.
func (m *Middleware) Exists(ctx context.Context, recipient util.HexAddress) (bool, error) {
	return m.HyperlaneApp.Exists(ctx, recipient)
}

// ReceiverIsmId dispatch the request to the underlying wrapped Hyperlane application.
func (m *Middleware) ReceiverIsmId(ctx context.Context, recipient util.HexAddress) (*util.HexAddress, error) {
	return m.HyperlaneApp.ReceiverIsmId(ctx, recipient)
}

// Handle allows to execute middleware hooks before and after the underlying wrapped application.
func (m *Middleware) Handle(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error {
	err := m.hook.PreHandleHook(ctx, mailboxID, message)
	if err != nil {
		return fmt.Errorf("pre-handle hook failed: %w", err)
	}

	err = m.HyperlaneApp.Handle(ctx, mailboxID, message)
	if err != nil {
		return fmt.Errorf("handler failed: %w", err)
	}

	err = m.hook.PostHandleHook(ctx, mailboxID, message)
	if err != nil {
		return fmt.Errorf("post-handle hook failed: %w", err)
	}
	return nil
}
