package app

// Debugging helpers:

var DebugFeatures = []struct {
	id      string
	enabled bool
}{
	{
		"reaper",
		false,
	},
	{
		"kubernetes-events-object-dumps",
		false,
	},
}
