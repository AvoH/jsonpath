package jsonpath

import (
	"context"
	"encoding/json"
	"fmt"
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

	pv := make([]interface{}, 0)
	ctx := context.WithValue(context.TODO(), MATCH_PATH_VALUE, &pv)

	_, err := Get("$.paths.*", data, ctx)
	if (err != nil) {
		fmt.Println(err.Error())
	}
	fmt.Println(pv)
}
