package jsonpath

import (
	"context"
)

type path interface {
	evaluate(c context.Context, parameter interface{}) (interface{}, error)
	visitMatchs(c context.Context, r interface{}, visit pathMatcher)
	withPlainSelector(plainSelector) path
	withAmbiguousSelector(ambiguousSelector) path
}

type plainPath []plainSelector

type ambiguousMatcher func(key interface{}, v *PathValue)

func (p plainPath) evaluate(ctx context.Context, root interface{}) (interface{}, error) {
	// return p.evaluatePath(ctx, root, root)
	var pv PathValue
	pv.Path = make([]string, 0)
	pv.Value = root
	pvRes, err := p.evaluatePath(ctx, root, &pv)
	return pvRes, err
}

func (p plainPath) evaluatePath(ctx context.Context, root interface{}, value *PathValue) (*PathValue, error) {
	var err error
	for _, sel := range p {
		value, err = sel(ctx, root, value)
		if err != nil {
			return nil, err
		}
	}
	return value, nil
}

func (p plainPath) matcher(ctx context.Context, r interface{}, match ambiguousMatcher) ambiguousMatcher {
	if len(p) == 0 {
		return match
	}
	return func(k interface{}, v *PathValue) {
		res, err := p.evaluatePath(ctx, r, v)
		if err == nil {
			match(k, res)
		}
	}
}

func (p plainPath) visitMatchs(ctx context.Context, r interface{}, visit pathMatcher) {
	var pv PathValue
	pv.Path = make([]string, 0)
	pv.Value = r
	res, err := p.evaluatePath(ctx, r, &pv)
	if err == nil {
		visit(nil, res)
	}
}

func (p plainPath) withPlainSelector(selector plainSelector) path {
	return append(p, selector)
}
func (p plainPath) withAmbiguousSelector(selector ambiguousSelector) path {
	return &ambiguousPath{
		parent: p,
		branch: selector,
	}
}

type ambiguousPath struct {
	parent path
	branch ambiguousSelector
	ending plainPath
}

func (p *ambiguousPath) evaluate(ctx context.Context, parameter interface{}) (interface{}, error) {
	matchs := []interface{}{}
	p.visitMatchs(ctx, parameter, func(keys []interface{}, match *PathValue) {
		matchs = append(matchs, *match)
	})
	return matchs, nil
}

func (p *ambiguousPath) visitMatchs(ctx context.Context, r interface{}, visit pathMatcher) {
	p.parent.visitMatchs(ctx, r, func(keys []interface{}, v *PathValue) {
		p.branch(ctx, r, v, p.ending.matcher(ctx, r, visit.matcher(keys)))
	})
}

func (p *ambiguousPath) branchMatcher(ctx context.Context, r interface{}, m ambiguousMatcher) ambiguousMatcher {
	return func(k interface{}, v *PathValue) {
		p.branch(ctx, r, v, m)
	}
}

func (p *ambiguousPath) withPlainSelector(selector plainSelector) path {
	p.ending = append(p.ending, selector)
	return p
}
func (p *ambiguousPath) withAmbiguousSelector(selector ambiguousSelector) path {
	return &ambiguousPath{
		parent: p,
		branch: selector,
	}
}

type pathMatcher func(keys []interface{}, match *PathValue)

func (m pathMatcher) matcher(keys []interface{}) ambiguousMatcher {
	return func(key interface{}, match *PathValue) {
		m(append(keys, key), match)
	}
}
