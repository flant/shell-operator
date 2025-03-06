package object_patch

import (
	sdkpkg "github.com/deckhouse/module-sdk/pkg"
)

type CreateOption func(o sdkpkg.PatchCollectorCreateOptionApplier)

func (opt CreateOption) Apply(o sdkpkg.PatchCollectorCreateOptionApplier) {
	opt(o)
}

func CreateWithSubresource(subresource string) CreateOption {
	return func(o sdkpkg.PatchCollectorCreateOptionApplier) {
		o.WithSubresource(subresource)
	}
}

type DeleteOption func(o sdkpkg.PatchCollectorDeleteOptionApplier)

func (opt DeleteOption) Apply(o sdkpkg.PatchCollectorDeleteOptionApplier) {
	opt(o)
}

func DeleteWithSubresource(subresource string) DeleteOption {
	return func(o sdkpkg.PatchCollectorDeleteOptionApplier) {
		o.WithSubresource(subresource)
	}
}

type PatchOption func(o sdkpkg.PatchCollectorPatchOptionApplier)

func (opt PatchOption) Apply(o sdkpkg.PatchCollectorPatchOptionApplier) {
	opt(o)
}

func PatchWithSubresource(subresource string) PatchOption {
	return func(o sdkpkg.PatchCollectorPatchOptionApplier) {
		o.WithSubresource(subresource)
	}
}

func PatchWithIgnoreMissingObject(ignore bool) PatchOption {
	return func(o sdkpkg.PatchCollectorPatchOptionApplier) {
		o.WithIgnoreMissingObject(ignore)
	}
}

func PatchWithIgnoreHookError(ignore bool) PatchOption {
	return func(o sdkpkg.PatchCollectorPatchOptionApplier) {
		o.WithIgnoreHookError(ignore)
	}
}

type FilterOption func(o sdkpkg.PatchCollectorFilterOptionApplier)

func (opt FilterOption) Apply(o sdkpkg.PatchCollectorFilterOptionApplier) {
	opt(o)
}

func FilterWithSubresource(subresource string) FilterOption {
	return func(o sdkpkg.PatchCollectorFilterOptionApplier) {
		o.WithSubresource(subresource)
	}
}

func FilterWithIgnoreMissingObject(ignore bool) FilterOption {
	return func(o sdkpkg.PatchCollectorFilterOptionApplier) {
		o.WithIgnoreMissingObject(ignore)
	}
}

func FilterWithIgnoreHookError(ignore bool) FilterOption {
	return func(o sdkpkg.PatchCollectorFilterOptionApplier) {
		o.WithIgnoreHookError(ignore)
	}
}
