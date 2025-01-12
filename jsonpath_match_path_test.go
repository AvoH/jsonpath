package jsonpath

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func setup(file string) map[string]interface{} {
	raw, _ := ioutil.ReadFile(file)
	var data map[string]interface{}
	json.Unmarshal(raw, &data)

	return data
}
func TestMatchedPath(t *testing.T)  {
	data := setup("petstore.json")

	result, err := Get("$.paths.*.*.parameters[*]", data)
	if (err != nil) {
		fmt.Println(err.Error())
	}

	switch res := result.(type) {
	case []interface{}:
		pathValues, _ := result.([]interface{})
		for _, pathValue := range pathValues {
			pv, ok := pathValue.(PathValue)
			assert.True(t, ok)

			fmt.Println(pv.Path)
			fmt.Println(pv.Value)
		}
	case interface{}:
		pv, ok := result.(*PathValue)
		assert.True(t, ok)

		fmt.Println(pv.Path)
		fmt.Println(pv.Value)
	default:
		fmt.Errorf("Unknown type %T of result", res)
	}
}