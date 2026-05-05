// Package json is a drop-in replacement for encoding/json that delegates to
// github.com/bytedance/sonic for better marshal/unmarshal performance.
// All public symbols required by the rest of the codebase are re-exported here
// so that callers only need to change their import path.
package json

import (
	stdjson "encoding/json"

	"github.com/bytedance/sonic"
)

type (
	Decoder               = sonic.Decoder
	Encoder               = sonic.Encoder
	Number                = stdjson.Number
	RawMessage            = stdjson.RawMessage
	Marshaler             = stdjson.Marshaler
	Unmarshaler           = stdjson.Unmarshaler
	InvalidUnmarshalError = stdjson.InvalidUnmarshalError
	UnmarshalTypeError    = stdjson.UnmarshalTypeError
	SyntaxError           = stdjson.SyntaxError
	MarshalerError        = stdjson.MarshalerError
)

var (
	Marshal       = sonic.Marshal
	MarshalIndent = sonic.MarshalIndent
	Unmarshal     = sonic.Unmarshal
	NewDecoder    = sonic.ConfigDefault.NewDecoder
	NewEncoder    = sonic.ConfigDefault.NewEncoder
	Valid         = sonic.Valid
)
