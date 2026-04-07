package config

import (
	"os"
)

func RocksDBInMemory() bool {
	return os.Getenv("ROCKSDB_IN_MEMORY") == "true"
}
