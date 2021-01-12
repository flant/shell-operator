package config

import (
	"testing"
)

func Test_GetSchema(t *testing.T) {
	schemas := []string{"v0", "v1"}

	for _, schema := range schemas {
		s := GetSchema(schema)
		if s == nil {
			t.Fatalf("schema '%s' should not be nil", schema)
		}
	}
}

func Test_LoadSchema_From_Schemas(t *testing.T) {
	for schemaVer := range Schemas {
		s, err := LoadSchema(schemaVer)
		if err != nil {
			t.Fatalf("schema '%s' should load: %v", schemaVer, err)
		}
		if s == nil {
			t.Fatalf("schema '%s' should not be nil: %v", schemaVer, err)
		}
	}
}
