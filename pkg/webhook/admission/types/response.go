package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

type AdmissionResponse struct {
	Allowed  bool     `json:"allowed"`
	Message  string   `json:"message,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Patch    []byte   `json:"patch,omitempty"`
}

func AdmissionResponseFromFile(filePath string) (*AdmissionResponse, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	return AdmissionResponseFromBytes(data)
}

func AdmissionResponseFromBytes(data []byte) (*AdmissionResponse, error) {
	return FromReader(bytes.NewReader(data))
}

func FromReader(r io.Reader) (*AdmissionResponse, error) {
	response := new(AdmissionResponse)

	dec := json.NewDecoder(r)
	if err := dec.Decode(response); err != nil {
		return nil, err
	}

	return response, nil
}

func (r *AdmissionResponse) Dump() string {
	b := new(strings.Builder)
	b.WriteString("ValidatingResponse(allowed=")
	b.WriteString(strconv.FormatBool(r.Allowed))
	if r.Message != "" {
		b.WriteString(",")
		b.WriteString(r.Message)
	}
	for _, warning := range r.Warnings {
		b.WriteString(",")
		b.WriteString(warning)
	}
	b.WriteString(")")
	return b.String()
}
