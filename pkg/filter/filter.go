package filter

type Filter interface {
	ApplyFilter(filterStr string, data map[string]any) ([]byte, error)
	FilterInfo() string
}

// CompiledFilter is a pre-compiled filter ready to be applied to data.
// Compile the filter expression once and reuse it across many Apply calls
// to avoid repeated parse/compile overhead.
type CompiledFilter interface {
	Apply(data map[string]any) ([]byte, error)
}
