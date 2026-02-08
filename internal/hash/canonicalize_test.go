package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalize_EmptyObject(t *testing.T) {
	c := NewCanonicalizer()
	result, err := c.Canonicalize(map[string]interface{}{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(result))
}

func TestCanonicalize_NilValue(t *testing.T) {
	c := NewCanonicalizer()
	result, err := c.Canonicalize(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "null", string(result))
}

func TestCanonicalize_KeysSorted(t *testing.T) {
	c := NewCanonicalizer()

	input := map[string]interface{}{
		"zebra": 1,
		"apple": 2,
		"mango": 3,
	}

	result, err := c.Canonicalize(input, nil)
	require.NoError(t, err)

	// Keys should be alphabetically sorted
	expected := `[{"k":"apple","v":2},{"k":"mango","v":3},{"k":"zebra","v":1}]`
	assert.Equal(t, expected, string(result))
}

func TestCanonicalize_DifferentKeyOrderSameResult(t *testing.T) {
	c := NewCanonicalizer()

	input1 := map[string]interface{}{"b": 2, "a": 1, "c": 3}
	input2 := map[string]interface{}{"c": 3, "a": 1, "b": 2}

	result1, err := c.Canonicalize(input1, nil)
	require.NoError(t, err)

	result2, err := c.Canonicalize(input2, nil)
	require.NoError(t, err)

	assert.Equal(t, string(result1), string(result2))
}

func TestCanonicalize_ExcludeFields(t *testing.T) {
	c := NewCanonicalizer()

	input := map[string]interface{}{
		"method":    "GET",
		"path":      "/api/test",
		"timestamp": "2024-01-01T00:00:00Z",
		"requestId": "abc-123",
	}

	result, err := c.Canonicalize(input, DefaultExcludeFields)
	require.NoError(t, err)

	resultStr := string(result)
	assert.Contains(t, resultStr, "method")
	assert.Contains(t, resultStr, "path")
	assert.NotContains(t, resultStr, "timestamp")
	assert.NotContains(t, resultStr, "requestId")
}

func TestCanonicalize_NestedObjects(t *testing.T) {
	c := NewCanonicalizer()

	input := map[string]interface{}{
		"outer": map[string]interface{}{
			"b": "second",
			"a": "first",
		},
	}

	result, err := c.Canonicalize(input, nil)
	require.NoError(t, err)

	// Nested map should also have sorted keys
	expected := `[{"k":"outer","v":[{"k":"a","v":"first"},{"k":"b","v":"second"}]}]`
	assert.Equal(t, expected, string(result))
}

func TestCanonicalize_NestedArrays(t *testing.T) {
	c := NewCanonicalizer()

	input := map[string]interface{}{
		"items": []interface{}{"c", "a", "b"},
	}

	result, err := c.Canonicalize(input, nil)
	require.NoError(t, err)

	// Array order should be preserved (not sorted)
	expected := `[{"k":"items","v":["c","a","b"]}]`
	assert.Equal(t, expected, string(result))
}

func TestCanonicalize_StringTrimming(t *testing.T) {
	c := NewCanonicalizer()

	input := map[string]interface{}{
		"name": "  hello world  ",
	}

	result, err := c.Canonicalize(input, nil)
	require.NoError(t, err)

	expected := `[{"k":"name","v":"hello world"}]`
	assert.Equal(t, expected, string(result))
}

func TestCanonicalize_SpecialCharacters(t *testing.T) {
	c := NewCanonicalizer()

	input := map[string]interface{}{
		"data": "hello \"world\" \n\ttab",
	}

	result, err := c.Canonicalize(input, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestCanonicalize_NilMapValues(t *testing.T) {
	c := NewCanonicalizer()

	input := map[string]interface{}{
		"key": nil,
	}

	result, err := c.Canonicalize(input, nil)
	require.NoError(t, err)

	expected := `[{"k":"key","v":null}]`
	assert.Equal(t, expected, string(result))
}

func TestCanonicalize_ExcludeFieldsCaseInsensitive(t *testing.T) {
	c := NewCanonicalizer()

	input := map[string]interface{}{
		"Timestamp": "2024-01-01T00:00:00Z",
		"method":    "GET",
	}

	result, err := c.Canonicalize(input, DefaultExcludeFields)
	require.NoError(t, err)

	resultStr := string(result)
	assert.NotContains(t, resultStr, "Timestamp")
	assert.Contains(t, resultStr, "method")
}
