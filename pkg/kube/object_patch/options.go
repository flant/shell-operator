package object_patch

func WithSubresource(str string) Subresource {
	return Subresource(str)
}

// Subresource is a configuration option for specifying a subresource.
type Subresource string

// ApplyToCreate applies this configuration to the given list options.
func (s Subresource) ApplyToCreate(opts *PatchCollectorCreateOptions) {
	opts.Subresource = string(s)
}

// ApplyToDelete applies this configuration to the given list options.
func (s Subresource) ApplyToDelete(opts *PatchCollectorDeleteOptions) {
	opts.Subresource = string(s)
}

// ApplyToPatch applies this configuration to the given list options.
func (s Subresource) ApplyToPatch(opts *PatchCollectorPatchOptions) {
	opts.Subresource = string(s)
}

func (s Subresource) ApplyToFilter(opts *PatchCollectorFilterOptions) {
	opts.Subresource = string(s)
}

func WithIgnoreHookError(ignore bool) IgnoreHookError {
	return IgnoreHookError(ignore)
}

// IgnoreHookError is a configuration option for ignoring hook errors.
type IgnoreHookError bool

// ApplyToPatch applies this configuration to the given list options.
func (i IgnoreHookError) ApplyToPatch(opts *PatchCollectorPatchOptions) {
	opts.IgnoreHookError = bool(i)
}

func (i IgnoreHookError) ApplyToFilter(opts *PatchCollectorFilterOptions) {
	opts.IgnoreHookError = bool(i)
}

func WithIgnoreMissingObject(ignore bool) IgnoreMissingObjects {
	return IgnoreMissingObjects(ignore)
}

// IgnoreMissingObjects is a configuration option for ignoring missing objects.
type IgnoreMissingObjects bool

// ApplyToPatch applies this configuration to the given list options.
func (m IgnoreMissingObjects) ApplyToPatch(opts *PatchCollectorPatchOptions) {
	opts.IgnoreMissingObjects = bool(m)
}

func (m IgnoreMissingObjects) ApplyToFilter(opts *PatchCollectorFilterOptions) {
	opts.IgnoreMissingObjects = bool(m)
}

// IgnoreIfExists is a configuration option for ignoring if object already exists.
type IgnoreIfExists bool

// ApplyToPatch applies this configuration to the given list options.
func (m IgnoreIfExists) ApplyToCreate(opts *PatchCollectorCreateOptions) {
	opts.IgnoreIfExists = bool(m)
}

// UpdateIfExists is an option for Create to update object if it already exists.
type UpdateIfExists bool

// ApplyToPatch applies this configuration to the given list options.
func (m UpdateIfExists) ApplyToCreate(opts *PatchCollectorCreateOptions) {
	opts.UpdateIfExists = bool(m)
}

type PatchCollectorCreateOption interface {
	ApplyToCreate(*PatchCollectorCreateOptions)
}

type PatchCollectorCreateOptions struct {
	Subresource    string
	IgnoreIfExists bool
	UpdateIfExists bool
}

// ApplyOptions applies the given list options on these options,
// and then returns itself (for convenient chaining).
func (o *PatchCollectorCreateOptions) ApplyOptions(opts []PatchCollectorCreateOption) *PatchCollectorCreateOptions {
	for _, opt := range opts {
		opt.ApplyToCreate(o)
	}

	return o
}

type PatchCollectorDeleteOption interface {
	ApplyToDelete(*PatchCollectorDeleteOptions)
}

type PatchCollectorDeleteOptions struct {
	Subresource string
}

// ApplyOptions applies the given list options on these options,
// and then returns itself (for convenient chaining).
func (o *PatchCollectorDeleteOptions) ApplyOptions(opts []PatchCollectorDeleteOption) *PatchCollectorDeleteOptions {
	for _, opt := range opts {
		opt.ApplyToDelete(o)
	}

	return o
}

type PatchCollectorPatchOption interface {
	ApplyToPatch(*PatchCollectorPatchOptions)
}

type PatchCollectorPatchOptions struct {
	Subresource          string
	IgnoreMissingObjects bool
	IgnoreHookError      bool
}

// ApplyOptions applies the given list options on these options,
// and then returns itself (for convenient chaining).
func (o *PatchCollectorPatchOptions) ApplyOptions(opts []PatchCollectorPatchOption) *PatchCollectorPatchOptions {
	for _, opt := range opts {
		opt.ApplyToPatch(o)
	}

	return o
}

type PatchCollectorFilterOption interface {
	ApplyToFilter(*PatchCollectorFilterOptions)
}

type PatchCollectorFilterOptions struct {
	Subresource          string
	IgnoreMissingObjects bool
	IgnoreHookError      bool
}

// ApplyOptions applies the given list options on these options,
// and then returns itself (for convenient chaining).
func (o *PatchCollectorFilterOptions) ApplyOptions(opts []PatchCollectorFilterOption) *PatchCollectorFilterOptions {
	for _, opt := range opts {
		opt.ApplyToFilter(o)
	}

	return o
}
