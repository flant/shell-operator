package filter

type Filter interface {
	ApplyFilter(filterStr string, data map[string]any) ([]byte, error)
	FilterInfo() string
}
