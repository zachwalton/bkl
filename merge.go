package bkl

import (
	"fmt"

	"golang.org/x/exp/slices"
)

func Merge(dst any, src any) (any, error) {
	switch dt := CanonicalizeType(dst).(type) {
	case map[string]any:
		return MergeMap(dt, src)

	case []any:
		return MergeList(dt, src)

	case nil:
		return src, nil

	default:
		return src, nil
	}
}

func MergeMap(dst map[string]any, src any) (any, error) {
	switch st := CanonicalizeType(src).(type) {
	case map[string]any:
		if patch, found := st["$patch"]; found {
			patchVal, ok := patch.(string)
			if !ok {
				return nil, fmt.Errorf("%T: %w", patch, ErrInvalidPatchType)
			}

			switch patchVal {
			case "replace":
				delete(st, "$patch")
				return st, nil

			default:
				return nil, fmt.Errorf("%s: %w", patch, ErrInvalidPatchValue)
			}
		}

		for k, v := range st {
			if v == nil {
				delete(dst, k)
				continue
			}

			existing, found := dst[k]
			if found {
				n, err := Merge(existing, v)
				if err != nil {
					return nil, fmt.Errorf("%s %w", k, err)
				}

				dst[k] = n
			} else {
				dst[k] = v
			}
		}

		return dst, nil

	case nil:
		return dst, nil

	default:
		return nil, fmt.Errorf("merge map[string]any with %T: %w", src, ErrInvalidType)
	}
}

func MergeList(dst []any, src any) (any, error) {
	switch st := CanonicalizeType(src).(type) {
	case []any:
		for i, v := range st {
			switch vt := CanonicalizeType(v).(type) { //nolint:gocritic
			case map[string]any:
				if patch, found := vt["$patch"]; found {
					patchVal, ok := patch.(string)
					if !ok {
						return nil, fmt.Errorf("%T: %w", patch, ErrInvalidPatchType)
					}

					switch patchVal {
					case "replace":
						return slices.Delete(st, i, i+1), nil

					default:
						return nil, fmt.Errorf("%s: %w", patch, ErrInvalidPatchValue)
					}
				}
			}

			dst = append(dst, v)
		}

		return dst, nil

	case nil:
		return dst, nil

	default:
		return nil, fmt.Errorf("merge []any with %T: %w", src, ErrInvalidType)
	}
}

func CanonicalizeType(in any) any {
	switch t := in.(type) {
	case []map[string]any:
		ret := []any{}
		for _, v := range t {
			ret = append(ret, v)
		}

		return ret

	default:
		return in
	}
}
