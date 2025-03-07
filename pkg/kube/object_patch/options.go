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

func WithIgnoreMissingObject(ignore bool) Option {
	return func(o sdkpkg.PatchCollectorOptionApplier) {
		o.WithIgnoreMissingObject(ignore)
	}
}

func WithIgnoreHookError(ignore bool) Option {
	return func(o sdkpkg.PatchCollectorOptionApplier) {
		o.WithIgnoreHookError(ignore)
	}
}
