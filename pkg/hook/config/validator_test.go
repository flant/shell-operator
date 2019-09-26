package config

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func Test_Validate_V1(t *testing.T) {
	data := `{
"configVrsion":"v1",
"schedule":{"name":"qwe"},
"qwdqwd":"QWD"
}`

	dataObj := make(map[string]interface{})
	e := json.Unmarshal([]byte(data), &dataObj)
	fmt.Printf("dataObj: %+v\nerr: %v\n", dataObj, e)

	res, err := ValidateConfig(dataObj, "v1", "root")

	assert.False(t, res)

	if assert.Error(t, err) {
		if merr, ok := err.(*multierror.Error); ok {
			for _, e := range merr.Errors {
				fmt.Printf("- %v\n", e)
			}
		}
	}
}
