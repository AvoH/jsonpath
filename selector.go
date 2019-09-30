package jsonpath

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/PaesslerAG/gval"
)

//plainSelector evaluate exactly one result
type plainSelector func(c context.Context, r interface{}, v pathValuePair) (pathValuePair, error)

//ambiguousSelector evaluate wildcard
type ambiguousSelector func(c context.Context, r interface{}, v pathValuePair, match ambiguousMatcher)

//@
func currentElementSelector() plainSelector {
	return func(c context.Context, r interface{}, v pathValuePair) (pathValuePair, error) {
		var pv pathValuePair
		pv.path = make([]string, 0)
		pv.value = c.Value(currentElement{})
		return pv, nil
	}
}

type currentElement struct{}

func currentContext(c context.Context, v interface{}) context.Context {
	return context.WithValue(c, currentElement{}, v)
}

//.x, [x]
func directSelector(key gval.Evaluable) plainSelector {
	return func(c context.Context, r interface{}, v pathValuePair) (pathValuePair, error) {
		var pv pathValuePair
		pv.path = append(v.path[:0:0], v.path...)
		e, matchedPath, err := selectValue(c, key, r, v.value)
		if err != nil {
			return v, err	// TODO: better way to return pathValuePair
		}
		pv.path = append(pv.path, matchedPath)
		pv.value = e
		return pv, nil
	}
}

// * / [*]
func starSelector() ambiguousSelector {
	return func(c context.Context, r interface{}, v pathValuePair, match ambiguousMatcher) {
		var pv pathValuePair
		pv.path = append(v.path[:0:0], v.path...)
		visitAll(v, func(key string, pval pathValuePair) {
			pv.path = append(pv.path, key)
			match(key, pval) })
	}
}

// [x, ...]
func multiSelector(keys []gval.Evaluable) ambiguousSelector {
	if len(keys) == 0 {
		return starSelector()
	}
	return func(c context.Context, r interface{}, v pathValuePair, match ambiguousMatcher) {
		for _, k := range keys {
			var pv pathValuePair
			pv.path = append(v.path[:0:0], v.path...)
			e, wildcard, err := selectValue(c, k, r, v.value)
			if err != nil {
				continue
			}
			pv.path = append(pv.path, wildcard)
			pv.value = e
			match(wildcard, pv)
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

func mapper(c context.Context, r interface{}, v pathValuePair, match ambiguousMatcher) {
	match([]interface{}{}, v)
	visitAll(v, func(wildcard string, v pathValuePair) {
		mapper(c, r, v, func(key interface{}, v pathValuePair) {
			match(append([]interface{}{wildcard}, key.([]interface{})...), v)
		})
	})
}

func visitAll(pv pathValuePair, visit func(key string, v pathValuePair)) {
	switch v := pv.value.(type) {
	case []interface{}:
		for i, e := range v {
			k := "[" + strconv.Itoa(i) + "]"
			var npv pathValuePair
			npv.path = append(pv.path[:0:0], pv.path...)
			npv.path = append(npv.path, k)
			npv.value = e
			visit(k, npv)
		}
	case map[string]interface{}:
		for k, e := range v {
			var npv pathValuePair
			npv.path = append(pv.path[:0:0], pv.path...)
			npv.path = append(npv.path, k)
			npv.value = e
			visit(k, npv)
		}
	}
}

//[? ]
func filterSelector(filter gval.Evaluable) ambiguousSelector {
	return func(c context.Context, r interface{}, v pathValuePair, match ambiguousMatcher) {
		visitAll(v, func(wildcard string, v pathValuePair) {
			var pv pathValuePair
			pv.path = append(v.path[:0:0], v.path...)
			pv.value = v.value
			condition, err := filter.EvalBool(currentContext(c, pv.value), r)
			if err != nil {
				return
			}
			if condition {
				match(wildcard, pv)
			}
		})
	}
}

//[::]
func rangeSelector(min, max, step gval.Evaluable) ambiguousSelector {
	return func(c context.Context, r interface{}, v pathValuePair, match ambiguousMatcher) {
		var pv pathValuePair
		pv.path = append(v.path[:0:0], v.path...)
		pv.value = v.value

		cs, ok := v.value.([]interface{})
		if !ok {
			return
		}

		c = currentContext(c, v.value)

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
				var pv pathValuePair
				pv.path = append(v.path[:0:0], v.path...)
				pv.value = cs[i]
				match(strconv.Itoa(i), pv)
			}
		} else {
			for i := max - 1; i >= min; i += step {
				var pv pathValuePair
				pv.path = append(v.path[:0:0], v.path...)
				pv.value = cs[i]
				match(strconv.Itoa(i), pv)
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
	return func(c context.Context, r interface{}, v pathValuePair) (pathValuePair, error) {
		val, err := script(currentContext(c, v.value), r)
		pv, ok := val.(pathValuePair)
		if (!ok) {
			return v, errors.New("script return non path value pair")	//TODO, refine error message
		}
		return pv, err
	}
}
