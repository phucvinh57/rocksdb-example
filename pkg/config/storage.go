package config

import (
	"os"
	"strings"
)

func MongoEnabled() bool {
	if envBool("IGNORE_MONGODB", false) {
		return false
	}

	return envBool("MONGODB_ENABLED", true)
}

func RocksDBInMemory() bool {
	return envBool("ROCKSDB_IN_MEMORY", false)
}

func envBool(key string, defaultValue bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}
