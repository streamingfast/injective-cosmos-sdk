package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type PostSendHook func(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coin) error

func (h PostSendHook) Then(second PostSendHook) PostSendHook {
	if h == nil {
		return second
	}
	if second == nil {
		return h
	}
	return func(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coin) error {
		if err := h(ctx, fromAddr, toAddr, amt); err != nil {
			return err
		}
		return second(ctx, fromAddr, toAddr, amt)
	}
}

// PostSendHookWrapper is a struct that houses a PostSendHook.
// It exists so that the PostSendHook can be updated in the SendKeeper without needing to have a pointer receiver.
type PostSendHookWrapper struct {
	fn PostSendHook
}

// NewPostSendHook creates a new post send hook wrapper with nil hook.
func NewPostSendHook() *PostSendHookWrapper {
	return &PostSendHookWrapper{
		fn: nil,
	}
}

// Append adds the provided hook to this, to be run after the existing function.
func (r *PostSendHookWrapper) Append(hook PostSendHook) {
	r.fn = r.fn.Then(hook)
}

// Prepend adds the provided hook to this, to be run before the existing function.
func (r *PostSendHookWrapper) Prepend(hook PostSendHook) {
	r.fn = hook.Then(r.fn)
}

// Clear removes the post-send hook (sets it to nil).
func (r *PostSendHookWrapper) Clear() {
	r.fn = nil
}

var _ PostSendHook = (*PostSendHookWrapper)(nil).Apply

// Apply runs the post-send hook if there is one. If not, it's a no-op.
func (r *PostSendHookWrapper) Apply(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coin) error {
	if r == nil || r.fn == nil {
		return nil
	}
	return r.fn(ctx, fromAddr, toAddr, amt)
}
