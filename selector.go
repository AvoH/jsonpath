package jsonpath

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/PaesslerAG/gval"
)

//plainSelector evaluate exactly one result
type plainSelector func(c context.Context, r interface{}, v *PathValue) (*PathValue, error)

//ambiguousSelector evaluate wildcard
type ambiguousSelector func(c context.Context, r interface{}, v *PathValue, match ambiguousMatcher)

//@
func currentElementSelector() plainSelector {
	return func(c context.Context, r interface{}, v *PathValue) (*PathValue, error) {
		var pv PathValue
		pv.Path = make([]string, 0)
		pv.Value = c.Value(currentElement{})
		return &pv, nil
	}
}

type currentElement struct{}

func currentContext(c context.Context, v interface{}) context.Context {
	return context.WithValue(c, currentElement{}, v)
}

//.x, [x]
func directSelector(key gval.Evaluable) plainSelector {
	return func(c context.Context, r interface{}, v *PathValue) (*PathValue, error) {
		var pv PathValue
		pv.Path = append(v.Path[:0:0], v.Path...)
		e, matchedPath, err := selectValue(c, key, r, v.Value)
		if err != nil {
			return v, err	// TODO: better way to return pathValuePair
		}
		pv.Path = append(pv.Path, matchedPath)
		pv.Value = e
		return &pv, nil
	}
}

// * / [*]
func starSelector() ambiguousSelector {
	return func(c context.Context, r interface{}, v *PathValue, match ambiguousMatcher) {
		var pv PathValue
		pv.Path = append(v.Path[:0:0], v.Path...)
		visitAll(v, func(key string, pval *PathValue) {
			pv.Path = append(pv.Path, key)
			match(key, pval) })
	}
}

// [x, ...]
func multiSelector(keys []gval.Evaluable) ambiguousSelector {
	if len(keys) == 0 {
		return starSelector()
	}
	return func(c context.Context, r interface{}, v *PathValue, match ambiguousMatcher) {
		for _, k := range keys {
			var pv PathValue
			pv.Path = append(v.Path[:0:0], v.Path...)
			e, wildcard, err := selectValue(c, k, r, v.Value)
			if err != nil {
				continue
			}
			pv.Path = append(pv.Path, wildcard)
			pv.Value = e
			match(wildcard, &pv)
		}
	}
}

func selectValue(c context.Context, key gval.Evaluable, r, v interface{}) (value interface{}, jkey string, err error) {
	c = currentContext(c, v)
	switch o := v.(type) {
	case []interface{}:
		i, err := key.EvalInt(c, r)
		if err != nil {
			return nil, "", fmt.Errorf("could not select value, invalid key: %s", err)
		}
		if i < 0 || i >= len(o) {
			return nil, "", fmt.Errorf("index %d out of bounds", i)
		}
		return o[i], strconv.Itoa(i), nil
	case map[string]interface{}:
		k, err := key.EvalString(c, r)
		if err != nil {
			return nil, "", fmt.Errorf("could not select value, invalid key: %s", err)
		}

		if r, ok := o[k]; ok {
			return r, k, nil
		}
		return nil, "", fmt.Errorf("unknown key %s", k)

	default:
		return nil, "", fmt.Errorf("unsupported value type %T for select, expected map[string]interface{} or []interface{}", o)
	}
}

//..
func mapperSelector() ambiguousSelector {
	return mapper
}

func mapper(c context.Context, r interface{}, v *PathValue, match ambiguousMatcher) {
	match([]interface{}{}, v)
	visitAll(v, func(wildcard string, v *PathValue) {
		mapper(c, r, v, func(key interface{}, v *PathValue) {
			match(append([]interface{}{wildcard}, key.([]interface{})...), v)
		})
	})
}

func visitAll(pv *PathValue, visit func(key string, v *PathValue)) {
	switch t := pv.Value.(type) {
	case []interface{}:
		values := pv.Value.([]interface{})
		for i, e := range values {
			k := "[" + strconv.Itoa(i) + "]"
			var npv PathValue
			npv.Path = append(pv.Path[:0:0], pv.Path...)
			npv.Path = append(npv.Path, k)
			npv.Value = e
			visit(k, &npv)
		}
	case map[string]interface{}:
		valueMap := pv.Value.(map[string]interface{})
		for k, e := range valueMap{
			var npv PathValue
			npv.Path = append(pv.Path[:0:0], pv.Path...)
			npv.Path = append(npv.Path, k)
			npv.Value = e
			visit(k, &npv)
		}
	default:
		fmt.Errorf("Invalid type %T in visitAll", t)
		return
	}
}

//[? ]
func filterSelector(filter gval.Evaluable) ambiguousSelector {
	return func(c context.Context, r interface{}, v *PathValue, match ambiguousMatcher) {
		visitAll(v, func(wildcard string, v *PathValue) {
			var pv PathValue
			pv.Path = append(v.Path[:0:0], v.Path...)
			pv.Value = v.Value
			condition, err := filter.EvalBool(currentContext(c, pv.Value), r)
			if err != nil {
				return
			}
			if condition {
				match(wildcard, &pv)
			}
		})
	}
}

//[::]
func rangeSelector(min, max, step gval.Evaluable) ambiguousSelector {
	return func(c context.Context, r interface{}, v *PathValue, match ambiguousMatcher) {
		var pv PathValue
		pv.Path = append(v.Path[:0:0], v.Path...)
		pv.Value = v.Value

		cs, ok := v.Value.([]interface{})
		if !ok {
			return
		}

		c = currentContext(c, v.Value)

		min, err := min.EvalInt(c, r)
		if err != nil {
			return
		}
		max, err := max.EvalInt(c, r)
		if err != nil {
			return
		}
		step, err := step.EvalInt(c, r)
		if err != nil {
			return
		}

		if min > max {
			return
		}

		n := len(cs)
		min = negmax(min, n)
		max = negmax(max, n)

		if step == 0 {
			step = 1
		}

		if step > 0 {
			for i := min; i < max; i += step {
				var pv PathValue
				pv.Path = append(v.Path[:0:0], v.Path...)
				pv.Value = cs[i]
				match(strconv.Itoa(i), &pv)
			}
		} else {
			for i := max - 1; i >= min; i += step {
				var pv PathValue
				pv.Path = append(v.Path[:0:0], v.Path...)
				pv.Value = cs[i]
				match(strconv.Itoa(i), &pv)
			}
		}

	}
}

func negmax(n, max int) int {
	if n < 0 {
		n = max + n
		if n < 0 {
			n = 0
		}
	} else if n > max {
		return max
	}
	return n
}

// ()
func newScript(script gval.Evaluable) plainSelector {
	return func(c context.Context, r interface{}, v *PathValue) (*PathValue, error) {
		val, err := script(currentContext(c, v.Value), r)
		pv, ok := val.(PathValue)
		if (!ok) {
			return nil, errors.New("script return non path value pair")	//TODO, refine error message
		}
		return &pv, err
	}
}
