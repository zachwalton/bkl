package bkl

import (
	"fmt"
	"strings"
)

func Process(obj, mergeFrom any, mergeFromDocs []any) (any, error) {
	return process(obj, mergeFrom, mergeFromDocs, 0)
}

func process(obj, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	if depth > 1000 {
		return nil, fmt.Errorf("%#v: %w", obj, ErrCircularRef)
	}

	switch objType := obj.(type) {
	case map[string]any:
		return processMap(objType, mergeFrom, mergeFromDocs, depth+1)

	case []any:
		return processList(objType, mergeFrom, mergeFromDocs, depth+1)

	case string:
		return processString(objType, mergeFrom, mergeFromDocs, depth+1)

	default:
		return obj, nil
	}
}

func processMap(obj map[string]any, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	m, obj := popMapValue(obj, "$merge")
	if m != nil {
		in, err := get(mergeFrom, mergeFromDocs, m)
		if err != nil {
			return nil, err
		}

		next, err := mergeMap(obj, in)
		if err != nil {
			return nil, err
		}

		return process(next, mergeFrom, mergeFromDocs, depth)
	}

	m, obj = popMapValue(obj, "$replace")
	if m != nil {
		next, err := get(mergeFrom, mergeFromDocs, m)
		if err != nil {
			return nil, err
		}

		return process(next, mergeFrom, mergeFromDocs, depth)
	}

	output, obj := popMapBoolValue(obj, "$output", false)
	if output {
		return nil, nil
	}

	encode, obj := popMapStringValue(obj, "$encode")

	obj, err := filterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, err := process(v, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			return nil, nil
		}

		return map[string]any{k: v2}, nil
	})
	if err != nil {
		return nil, err
	}

	if encode != "" {
		f, err := GetFormat(encode)
		if err != nil {
			return nil, err
		}

		enc, err := f.Marshal(obj)
		if err != nil {
			return nil, err
		}

		return string(enc), nil
	}

	return obj, nil
}

func processList(obj []any, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	path, obj, err := popListMapStringValue(obj, "$merge")
	if err != nil {
		return nil, err
	}

	if path != "" {
		in, err := get(mergeFrom, mergeFromDocs, path)
		if err != nil {
			return nil, err
		}

		next, err := mergeList(obj, in)
		if err != nil {
			return nil, err
		}

		return process(next, mergeFrom, mergeFromDocs, depth)
	}

	path, obj, err = popListMapStringValue(obj, "$replace")
	if err != nil {
		return nil, err
	}

	if path != "" {
		next, err := get(mergeFrom, mergeFromDocs, path)
		if err != nil {
			return nil, err
		}

		return process(next, mergeFrom, mergeFromDocs, depth)
	}

	if hasListMapBoolValue(obj, "$output", false) {
		return nil, nil
	}

	encode, obj, err := popListMapStringValue(obj, "$encode")
	if err != nil {
		return nil, err
	}

	obj, err = filterList(obj, func(v any) ([]any, error) {
		v2, err := process(v, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			return nil, nil
		}

		return []any{v2}, nil
	})
	if err != nil {
		return nil, err
	}

	if encode != "" {
		f, found := formatByExtension[encode]
		if !found {
			return nil, fmt.Errorf("%s: %w", encode, ErrUnknownFormat)
		}

		enc, err := f.Marshal(obj)
		if err != nil {
			return nil, err
		}

		return string(enc), nil
	}

	return obj, nil
}

func processString(obj string, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	if strings.HasPrefix(obj, "$merge:") {
		path := strings.TrimPrefix(obj, "$merge:")

		in, err := get(mergeFrom, mergeFromDocs, path)
		if err != nil {
			return nil, err
		}

		return process(in, mergeFrom, mergeFromDocs, depth)
	}

	if strings.HasPrefix(obj, "$replace:") {
		path := strings.TrimPrefix(obj, "$replace:")

		in, err := get(mergeFrom, mergeFromDocs, path)
		if err != nil {
			return nil, err
		}

		return process(in, mergeFrom, mergeFromDocs, depth)
	}

	return obj, nil
}
