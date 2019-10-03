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

	s := GetSchema("v1")

	err := ValidateConfig(dataObj, s, "root")

	if assert.Error(t, err) {
		assert.IsType(t, &multierror.Error{}, err)
		t.Logf("expected multierror was: %v", err)
	}
}
