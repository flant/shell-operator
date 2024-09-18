package admission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type Response struct {
	Allowed  bool     `json:"allowed"`
	Message  string   `json:"message,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Patch    []byte   `json:"patch,omitempty"`
}

func ResponseFromFile(filePath string) (*Response, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	return ResponseFromBytes(data)
}

func ResponseFromBytes(data []byte) (*Response, error) {
	return FromReader(bytes.NewReader(data))
}

func FromReader(r io.Reader) (*Response, error) {
	response := new(Response)

	if err := json.NewDecoder(r).Decode(response); err != nil {
		return nil, err
	}

	return response, nil
}

func (r *Response) Dump() string {
	b := new(strings.Builder)
	b.WriteString("AdmissionResponse(allowed=")
	b.WriteString(strconv.FormatBool(r.Allowed))
	if len(r.Patch) > 0 {
		b.WriteString(",patch=")
		b.Write(r.Patch)
	}
	if r.Message != "" {
		b.WriteString(",msg=")
		b.WriteString(r.Message)
	}
	for _, warning := range r.Warnings {
		b.WriteString(",warn=")
		b.WriteString(warning)
	}
	b.WriteString(")")
	return b.String()
}
