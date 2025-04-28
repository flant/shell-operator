package filter

type Filter interface {
	ApplyFilter(filterStr string, data []byte) (string, error)
	FilterInfo() string
}
