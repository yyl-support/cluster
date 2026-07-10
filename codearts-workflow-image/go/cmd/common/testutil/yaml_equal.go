package testutil

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

func YamlEqual(a, b []byte) (bool, error) {
	var dataA, dataB interface{}

	if err := yaml.Unmarshal(a, &dataA); err != nil {
		return false, err
	}
	if err := yaml.Unmarshal(b, &dataB); err != nil {
		return false, err
	}

	dataA = normalizeYAML(dataA)
	dataB = normalizeYAML(dataB)

	return reflect.DeepEqual(dataA, dataB), nil
}

func normalizeYAML(v interface{}) interface{} {
	switch val := v.(type) {
	case []interface{}:
		for i := range val {
			val[i] = normalizeYAML(val[i])
		}
		sort.Slice(val, func(i, j int) bool {
			return fmt.Sprintf("%v", val[i]) < fmt.Sprintf("%v", val[j])
		})
		return val
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sorted := make(map[string]interface{}, len(val))
		for _, k := range keys {
			sorted[k] = normalizeYAML(val[k])
		}
		return sorted
	case string:
		return strings.TrimRight(val, "\n")
	default:
		return val
	}
}
