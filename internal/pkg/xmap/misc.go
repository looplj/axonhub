package xmap

import (
	"github.com/samber/lo"
)

// GetStringPtr extracts a *string value from a map[string]any.
func GetStringPtr(m map[string]any, key string) *string {
	if m == nil {
		return nil
	}

	if v, ok := m[key]; ok {
		switch vv := v.(type) {
		case string:
			return lo.ToPtr(vv)
		case *string:
			return vv
		default:
			return nil
		}
	}

	return nil
}

// GetInt64Ptr extracts a *int64 value from a map[string]any.
func GetInt64Ptr(m map[string]any, key string) *int64 {
	if m == nil {
		return nil
	}

	if v, ok := m[key]; ok {
		switch vv := v.(type) {
		case int64:
			return lo.ToPtr(vv)
		case *int64:
			return vv
		default:
			return nil
		}
	}

	return nil
}

// GetBoolPtr extracts a *bool value from a map[string]any.
func GetBoolPtr(m map[string]any, key string) *bool {
	if m == nil {
		return nil
	}

	if v, ok := m[key]; ok {
		switch vv := v.(type) {
		case bool:
			return lo.ToPtr(vv)
		case *bool:
			return vv
		default:
			return nil
		}
	}

	return nil
}

// GetStringSlice extracts a []string value from a map[string]any.
func GetStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}

	if v, ok := m[key]; ok {
		if slice, ok := v.([]string); ok {
			return slice
		}
	}

	return nil
}

func GetSlice[T any](m map[string]any, key string) []T {
	if m == nil {
		return nil
	}

	if v, ok := m[key]; ok {
		switch vv := v.(type) {
		case []T:
			return vv
		case []*T:
			return lo.FromSlicePtr(vv)
		default:
			return []T{}
		}
	}

	return nil
}

func GetSlicePtr[T any](m map[string]any, key string) []*T {
	if m == nil {
		return nil
	}

	if v, ok := m[key]; ok {
		switch vv := v.(type) {
		case []*T:
			return vv
		case []T:
			return lo.ToSlicePtr(vv)
		default:
			return nil
		}
	}

	return nil
}

func GetPtr[T any](m map[string]any, key string) *T {
	if m == nil {
		return nil
	}

	if v, ok := m[key]; ok {
		switch vv := v.(type) {
		case T:
			return lo.ToPtr(vv)
		case *T:
			return vv
		default:
			return nil
		}
	}

	return nil
}
