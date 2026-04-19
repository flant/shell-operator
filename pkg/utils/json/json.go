// Package json is a drop-in replacement for encoding/json that delegates to
// github.com/goccy/go-json for better marshal/unmarshal performance.
// All public symbols required by the rest of the codebase are re-exported here
// so that callers only need to change their import path.
package json

import (
	gojson "github.com/goccy/go-json"
)

type (
	Decoder              = gojson.Decoder
	Encoder              = gojson.Encoder
	Number               = gojson.Number
	RawMessage           = gojson.RawMessage
	Marshaler            = gojson.Marshaler
	Unmarshaler          = gojson.Unmarshaler
	InvalidUnmarshalError = gojson.InvalidUnmarshalError
	UnmarshalTypeError   = gojson.UnmarshalTypeError
	SyntaxError          = gojson.SyntaxError
	MarshalerError       = gojson.MarshalerError
)

var (
	Marshal       = gojson.Marshal
	MarshalIndent = gojson.MarshalIndent
	Unmarshal     = gojson.Unmarshal
	NewDecoder    = gojson.NewDecoder
	NewEncoder    = gojson.NewEncoder
	Valid         = gojson.Valid
)
