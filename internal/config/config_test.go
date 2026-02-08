package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any NEXUS_ env vars that might interfere
	os.Unsetenv("NEXUS_PORT")
	os.Unsetenv("NEXUS_MONGO_URI")
	os.Unsetenv("NEXUS_DATABASE_NAME")
	os.Unsetenv("NEXUS_COLLECTION_NAME")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "nexus", cfg.DatabaseName)
	assert.Equal(t, "snapshots", cfg.Collection)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 10, cfg.ReadTimeout)
	assert.Equal(t, 10, cfg.WriteTimeout)
	assert.Equal(t, 120, cfg.IdleTimeout)
}

func TestLoad_CustomValues(t *testing.T) {
	os.Setenv("NEXUS_PORT", "9090")
	os.Setenv("NEXUS_MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("NEXUS_DATABASE_NAME", "testdb")
	os.Setenv("NEXUS_COLLECTION_NAME", "testcol")
	defer func() {
		os.Unsetenv("NEXUS_PORT")
		os.Unsetenv("NEXUS_MONGO_URI")
		os.Unsetenv("NEXUS_DATABASE_NAME")
		os.Unsetenv("NEXUS_COLLECTION_NAME")
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "mongodb://localhost:27017", cfg.MongoURI)
	assert.Equal(t, "testdb", cfg.DatabaseName)
	assert.Equal(t, "testcol", cfg.Collection)
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{
		Port:         0,
		MongoURI:     "mongodb://localhost:27017",
		DatabaseName: "nexus",
		Collection:   "snapshots",
	}
	err := cfg.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port must be between")
}

func TestValidate_PortTooHigh(t *testing.T) {
	cfg := &Config{
		Port:         70000,
		MongoURI:     "mongodb://localhost:27017",
		DatabaseName: "nexus",
		Collection:   "snapshots",
	}
	err := cfg.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port must be between")
}

func TestValidate_EmptyMongoURI(t *testing.T) {
	cfg := &Config{
		Port:         8080,
		MongoURI:     "",
		DatabaseName: "nexus",
		Collection:   "snapshots",
	}
	err := cfg.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mongo URI must not be empty")
}

func TestValidate_EmptyDatabaseName(t *testing.T) {
	cfg := &Config{
		Port:         8080,
		MongoURI:     "mongodb://localhost:27017",
		DatabaseName: "",
		Collection:   "snapshots",
	}
	err := cfg.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database name must not be empty")
}

func TestValidate_EmptyCollection(t *testing.T) {
	cfg := &Config{
		Port:         8080,
		MongoURI:     "mongodb://localhost:27017",
		DatabaseName: "nexus",
		Collection:   "",
	}
	err := cfg.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collection name must not be empty")
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Port:         8080,
		MongoURI:     "mongodb://localhost:27017",
		DatabaseName: "nexus",
		Collection:   "snapshots",
	}
	err := cfg.validate()
	assert.NoError(t, err)
}
