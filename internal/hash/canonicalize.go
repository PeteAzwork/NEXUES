package hash

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DefaultExcludeFields are fields stripped before hashing.
var DefaultExcludeFields = []string{"timestamp", "requestId", "requestID"}

// Canonicalizer converts arbitrary data into a deterministic byte representation.
type Canonicalizer interface {
	Canonicalize(v interface{}, excludeFields []string) ([]byte, error)
}

// JSONCanonicalizer produces deterministic JSON by sorting keys and trimming strings.
type JSONCanonicalizer struct{}

// NewCanonicalizer returns a new JSONCanonicalizer.
func NewCanonicalizer() *JSONCanonicalizer {
	return &JSONCanonicalizer{}
}

// Canonicalize returns a deterministic byte representation of v with excluded fields removed.
func (c *JSONCanonicalizer) Canonicalize(v interface{}, excludeFields []string) ([]byte, error) {
	// Convert to a generic representation via JSON round-trip.
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal for canonicalization: %w", err)
	}

	var generic interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("unmarshal for canonicalization: %w", err)
	}

	excludeSet := make(map[string]bool, len(excludeFields))
	for _, f := range excludeFields {
		excludeSet[strings.ToLower(f)] = true
	}

	cleaned := cleanValue(generic, excludeSet)
	return json.Marshal(cleaned)
}

// cleanValue recursively sorts map keys, trims strings, and removes excluded fields.
func cleanValue(v interface{}, excludeFields map[string]bool) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]interface{}:
		return cleanMap(val, excludeFields)
	case []interface{}:
		return cleanSlice(val, excludeFields)
	case string:
		return strings.TrimSpace(val)
	default:
		return val
	}
}

// cleanMap produces an ordered representation of a map with excluded fields removed.
func cleanMap(m map[string]interface{}, excludeFields map[string]bool) interface{} {
	type kv struct {
		Key   string      `json:"k"`
		Value interface{} `json:"v"`
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		if excludeFields[strings.ToLower(k)] {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]kv, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, kv{Key: k, Value: cleanValue(m[k], excludeFields)})
	}
	return pairs
}

// cleanSlice recursively cleans each element of a slice.
func cleanSlice(s []interface{}, excludeFields map[string]bool) interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = cleanValue(v, excludeFields)
	}
	return result
}
