package data

import (
	"fmt"
	"gopkg.in/yaml.v2"
)

// Converts YAML structure to string.
// Can be used for error formating. Ex.:
// fmt.Errorf("expected map at key 'global', got:\n%s", utils.YamlToString(globalValuesRaw))
//
// !! Panic if data not converted to yaml to speed up detecting problems. !!
func YamlToString(data interface{}) string {
	valuesYaml, err := yaml.Marshal(&data)
	if err != nil {
		panic(fmt.Sprintf("Cannot dump data to YAML: \n%#v\n error: %s", data, err))
	}
	return string(valuesYaml)
}
