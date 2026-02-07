package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate_SameInputSameHash(t *testing.T) {
	g := NewGenerator()

	request := map[string]interface{}{"method": "GET", "path": "/api/test"}
	response := map[string]interface{}{"statusCode": 200}
	attributes := map[string]interface{}{"subject": "user1"}

	hash1, err := g.Generate(request, response, attributes)
	require.NoError(t, err)

	hash2, err := g.Generate(request, response, attributes)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}

func TestGenerate_DifferentInputDifferentHash(t *testing.T) {
	g := NewGenerator()

	request1 := map[string]interface{}{"method": "GET", "path": "/api/test"}
	request2 := map[string]interface{}{"method": "POST", "path": "/api/test"}
	response := map[string]interface{}{"statusCode": 200}
	attributes := map[string]interface{}{}

	hash1, err := g.Generate(request1, response, attributes)
	require.NoError(t, err)

	hash2, err := g.Generate(request2, response, attributes)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestGenerate_FieldOrderDoesNotAffectHash(t *testing.T) {
	g := NewGenerator()

	request1 := map[string]interface{}{"method": "GET", "path": "/api/test", "body": "data"}
	request2 := map[string]interface{}{"body": "data", "method": "GET", "path": "/api/test"}
	response := map[string]interface{}{"statusCode": 200}
	attributes := map[string]interface{}{}

	hash1, err := g.Generate(request1, response, attributes)
	require.NoError(t, err)

	hash2, err := g.Generate(request2, response, attributes)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}

func TestGenerate_ExcludedFieldsDontAffectHash(t *testing.T) {
	g := NewGenerator()

	request1 := map[string]interface{}{
		"method": "GET",
		"path":   "/api/test",
	}
	request2 := map[string]interface{}{
		"method":    "GET",
		"path":      "/api/test",
		"timestamp": "2024-01-01T00:00:00Z",
		"requestId": "abc-123",
	}
	response := map[string]interface{}{"statusCode": 200}
	attributes := map[string]interface{}{}

	hash1, err := g.Generate(request1, response, attributes)
	require.NoError(t, err)

	hash2, err := g.Generate(request2, response, attributes)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}

func TestGenerate_HexEncoded(t *testing.T) {
	g := NewGenerator()

	request := map[string]interface{}{"method": "GET"}
	response := map[string]interface{}{"statusCode": 200}
	attributes := map[string]interface{}{}

	hash, err := g.Generate(request, response, attributes)
	require.NoError(t, err)

	// SHA-256 produces 64 hex characters
	assert.Len(t, hash, 64)

	// Should only contain hex characters
	for _, c := range hash {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"hash should only contain hex characters, got: %c", c)
	}
}

func TestGenerate_DifferentResponseDifferentHash(t *testing.T) {
	g := NewGenerator()

	request := map[string]interface{}{"method": "GET", "path": "/api/test"}
	response1 := map[string]interface{}{"statusCode": 200}
	response2 := map[string]interface{}{"statusCode": 404}
	attributes := map[string]interface{}{}

	hash1, err := g.Generate(request, response1, attributes)
	require.NoError(t, err)

	hash2, err := g.Generate(request, response2, attributes)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestGenerate_DifferentAttributesDifferentHash(t *testing.T) {
	g := NewGenerator()

	request := map[string]interface{}{"method": "GET", "path": "/api/test"}
	response := map[string]interface{}{"statusCode": 200}
	attributes1 := map[string]interface{}{"subject": "user1"}
	attributes2 := map[string]interface{}{"subject": "user2"}

	hash1, err := g.Generate(request, response, attributes1)
	require.NoError(t, err)

	hash2, err := g.Generate(request, response, attributes2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestGenerate_NilInputs(t *testing.T) {
	g := NewGenerator()

	hash, err := g.Generate(nil, nil, nil)
	require.NoError(t, err)
	assert.Len(t, hash, 64)
}
