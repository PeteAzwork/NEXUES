package hash

import (
	"crypto/sha256"
	"fmt"
)

// Generator produces deterministic hashes from request/response/attribute data.
type Generator interface {
	Generate(request, response, attributeState interface{}) (string, error)
}

// SHA256Generator implements Generator using SHA-256 with canonicalization.
type SHA256Generator struct {
	canonicalizer Canonicalizer
	excludeFields []string
}

// NewGenerator returns a new SHA256Generator.
func NewGenerator() *SHA256Generator {
	return &SHA256Generator{
		canonicalizer: NewCanonicalizer(),
		excludeFields: DefaultExcludeFields,
	}
}

// Generate produces a hex-encoded SHA-256 hash from the combined canonical forms
// of request, response, and attributeState.
func (g *SHA256Generator) Generate(request, response, attributeState interface{}) (string, error) {
	reqBytes, err := g.canonicalizer.Canonicalize(request, g.excludeFields)
	if err != nil {
		return "", fmt.Errorf("canonicalizing request: %w", err)
	}

	respBytes, err := g.canonicalizer.Canonicalize(response, g.excludeFields)
	if err != nil {
		return "", fmt.Errorf("canonicalizing response: %w", err)
	}

	attrBytes, err := g.canonicalizer.Canonicalize(attributeState, g.excludeFields)
	if err != nil {
		return "", fmt.Errorf("canonicalizing attribute state: %w", err)
	}

	h := sha256.New()
	h.Write(reqBytes)
	h.Write([]byte("|"))
	h.Write(respBytes)
	h.Write([]byte("|"))
	h.Write(attrBytes)

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
