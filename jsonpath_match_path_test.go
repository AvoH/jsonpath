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

	result, err := Get("$.paths.*.*.parameters.*.name", data)
	if (err != nil) {
		fmt.Println(err.Error())
	}

	switch res := result.(type) {
	case []interface{}:
		pathValues, _ := result.([]interface{})
		for _, pathValue := range pathValues {
			pv, ok := pathValue.(pathValuePair)
			assert.True(t, ok)

			fmt.Println(pv.path)
			fmt.Println(pv.value)
		}
	case interface{}:
		pv, ok := result.(pathValuePair)
		assert.True(t, ok)

		fmt.Println(pv.path)
		fmt.Println(pv.value)
	default:
		fmt.Errorf("Unknown type %T of result", res)
	}
}
