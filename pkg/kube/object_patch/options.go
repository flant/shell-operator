package object_patch

import (
	sdkpkg "github.com/deckhouse/module-sdk/pkg"
)

type Option func(o sdkpkg.PatchCollectorOptionApplier)

func (opt Option) Apply(o sdkpkg.PatchCollectorOptionApplier) {
	opt(o)
}

func WithSubresource(subresource string) Option {
	return func(o sdkpkg.PatchCollectorOptionApplier) {
		o.WithSubresource(subresource)
	}
}

func withIgnoreMissingObject(ignore bool) Option {
	return func(o sdkpkg.PatchCollectorOptionApplier) {
		o.WithIgnoreMissingObject(ignore)
	}
}

func WithIgnoreMissingObject() Option {
	return withIgnoreMissingObject(true)
}

func withIgnoreHookError(ignore bool) Option {
	return func(o sdkpkg.PatchCollectorOptionApplier) {
		o.WithIgnoreHookError(ignore)
	}
}

func WithIgnoreHookError() Option {
	return withIgnoreHookError(true)
}
