package bkl

import (
	"errors"
	"fmt"
	"strings"
)

func process(root any) (any, error) {
	return processRecursive(root, root)
}

func processRecursive(root any, obj any) (any, error) {
	switch objType := obj.(type) {
	case map[string]any:
		return processMap(root, objType)

	case []any:
		return processList(root, objType)

	case string:
		return processString(root, objType)

	default:
		return obj, nil
	}
}

func processMap(root any, obj map[string]any) (any, error) {
	path, obj := popStringValue(obj, "$merge")
	if path != "" {
		in := get(root, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		next, err := mergeMap(obj, in)
		if err != nil {
			return nil, err
		}

		return processRecursive(root, next)
	}

	path, obj = popStringValue(obj, "$replace")
	if path != "" {
		next := get(root, path)
		if next == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrReplaceRefNotFound)
		}

		return processRecursive(root, next)
	}

	output, obj := popBoolValue(obj, "$output", false)
	if output {
		return nil, nil
	}

	encode, obj := popStringValue(obj, "$encode")

	obj, err := filterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, err := processRecursive(root, v)
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
		f, err := getFormat(encode)
		if err != nil {
			return nil, err
		}

		enc, err := f.encode(obj)
		if err != nil {
			return nil, errors.Join(ErrEncode, err)
		}

		return string(enc), nil
	}

	return obj, nil
}

func processList(root any, obj []any) (any, error) {
	if listHasBoolValue(obj, "$output", false) {
		return nil, nil
	}

	// TODO: Support $merge, $replace

	encode, obj := listPopStringValue(obj, "$encode")

	obj, err := filterList(obj, func(v any) ([]any, error) {
		v2, err := processRecursive(root, v)
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

		enc, err := f.encode(obj)
		if err != nil {
			return nil, errors.Join(ErrEncode, err)
		}

		return string(enc), nil
	}

	return obj, nil
}

func processString(root any, obj string) (any, error) {
	if strings.HasPrefix(obj, "$merge:") {
		path := strings.TrimPrefix(obj, "$merge:")

		in := get(root, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		return processRecursive(root, in)
	}

	if strings.HasPrefix(obj, "$replace:") {
		path := strings.TrimPrefix(obj, "$replace:")

		in := get(root, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		return processRecursive(root, in)
	}

	return obj, nil
}
