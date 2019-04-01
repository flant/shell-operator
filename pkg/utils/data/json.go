package data

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func FormatJsonDataOrError(jsonData string, err error) string {
	if err != nil {
		return fmt.Sprintf("<bad json: %s>", err)
	}
	return jsonData
}

func FormatPrettyJson(jsonData string) (string, error) {
	var out bytes.Buffer

	err := json.Indent(&out, []byte(jsonData), "", "  ")
	if err != nil {
		return "", fmt.Errorf("bad json data: %s", err)
	}

	return out.String(), nil
}
