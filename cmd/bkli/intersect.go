package main

import (
	"strings"
)

type op int

const (
	intersection op = iota
	difference
)

var (
	jsonPathListMatchers = []string{
		"name",
		"metadata.name",
	}
)

func subtract(from, by any) (any, error) {
	return walk("$<subtract>", from, by, difference)
}

func intersect(a, b any) (any, error) {
	return walk("$<intersect>", a, b, intersection)
}

func walk(path string, a, b any, o op) (any, error) {
	switch a2 := a.(type) {
	case map[string]any:
		return walkMap(path, a2, b, o)

	case []any:
		return walkList(path, a2, b, o)

	case nil:
		return nil, nil

	default:
		if a == b {
			switch o {
			case intersection:
				return a, nil
			case difference:
				return nil, nil
			}
		}

		return requiredOrMinuend(a, b, o), nil
	}
}

func walkMap(path string, a map[string]any, b any, o op) (any, error) {
	switch b2 := b.(type) {
	case map[string]any:
		return walkMapMap(path, a, b2, o)
	default:
		// Different types but both defined
		return requiredOrMinuend(a, b, o), nil
	}
}

func walkMapMap(path string, a, b map[string]any, o op) (map[string]any, error) {
	ret := map[string]any{}

	for k, v := range a {
		v2, found := b[k]
		if !found {
			continue
		}

		if v == nil && v2 == nil && o == difference {
			continue
		}

		v3, err := walk(path+"."+k, v, v2, o)
		if err != nil {
			return nil, err
		}

		if v3 == nil {
			continue
		}

		ret[k] = v3
	}

	return ret, nil
}

func walkList(path string, a []any, b any, o op) (any, error) {
	switch b2 := b.(type) {
	case []any:
		ret, err := walkListList(path, a, b2, o)
		if err != nil {
			return nil, err
		}
		switch o {
		case difference:
			if len(ret) == 0 {
				return nil, nil
			}
		default:
			return ret, nil
		}
	}
	// Different types but both defined
	return requiredOrMinuend(a, b, o), nil
}

func walkListList(path string, a, b []any, o op) ([]any, error) { //nolint:unparam
	var (
		rets []any
		ret  any
		err  error
	)

	for i, v1 := range a {
		matchedMap := false
		for _, v2 := range b {
			if listEntryMatches(path, v1, v2) {
				ret, err = walkMapMap(path, v1.(map[string]any), v2.(map[string]any), o)
				if err != nil {
					return nil, err
				}
				matchedMap = true
				break
			}
		}

		if matchedMap {
			if ret == nil {
				continue
			}
			rets = append(rets, ret)
			continue
		}
		if i >= len(b) {
			rets = append(rets, ret)
			continue
		}
		ret, err = walk(path, v1, b[i], o)
		if err != nil {
			return nil, err
		}
		if ret == nil {
			continue
		}
		rets = append(rets, ret)
	}

	if len(rets) == 0 {
		if o == difference {
			return nil, nil
		}
		rets = append(rets, "$required")
	}

	return rets, nil
}

func requiredOrMinuend(a, b any, o op) any {
	switch o {
	case intersection:
		return "$required"
	default:
		return a
	}
}

func listEntryMatches(path, a, b any) bool {
	var (
		origA, origB = a, b
		m1, m2       map[string]any
		m1ok, m2ok   bool
	)
	for _, p := range jsonPathListMatchers {
		for _, k := range strings.Split(p, ".") {
			m1, m1ok = a.(map[string]any)
			m2, m2ok = b.(map[string]any)

			if !m1ok || !m2ok {
				return false
			}

			a, m1ok = m1[k]
			b, m2ok = m2[k]

			if !m1ok || !m2ok {
				a, b = origA, origB
				break
			}
		}

		if m1ok && m2ok && a == b {
			return true
		}
	}

	return false
}
