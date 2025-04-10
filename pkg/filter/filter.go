package filter

type Filter interface {
	ApplyFilter(filterStr string, data map[string]any) (map[string]any, error)
	FilterInfo() string
}
