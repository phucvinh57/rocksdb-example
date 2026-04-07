package rocksdb

import (
	"fmt"
	"os"
	"time"
	"trading-bsx/pkg/config"

	"github.com/linxGnu/grocksdb"
)

var BuyOrder *grocksdb.DB
var SellOrder *grocksdb.DB
var env *grocksdb.Env

func Init() {
	if BuyOrder != nil {
		BuyOrder.Close()
		BuyOrder = nil
	}
	if SellOrder != nil {
		SellOrder.Close()
		SellOrder = nil
	}
	if env != nil {
		env.Destroy()
		env = nil
	}

	cwd, _ := os.Getwd()

	bookName := ""
	if os.Getenv("ENV") == "test" {
		bookName = fmt.Sprintf("test_%d_", time.Now().UnixMilli())
	}

	bbto := grocksdb.NewDefaultBlockBasedTableOptions()
	bbto.SetBlockCache(grocksdb.NewLRUCache(3 << 30))

	opts := grocksdb.NewDefaultOptions()
	opts.SetBlockBasedTableFactory(bbto)
	opts.SetCreateIfMissing(true)

	buyOrderPath := fmt.Sprintf("%s/rocksdb_data/%sbuy_order", cwd, bookName)
	sellOrderPath := fmt.Sprintf("%s/rocksdb_data/%ssell_order", cwd, bookName)
	if config.RocksDBInMemory() {
		env = grocksdb.NewMemEnv()
		opts.SetEnv(env)
		buyOrderPath = fmt.Sprintf("%sbuy_order", bookName)
		sellOrderPath = fmt.Sprintf("%ssell_order", bookName)
	} else {
		os.MkdirAll(buyOrderPath, os.ModePerm)
		os.MkdirAll(sellOrderPath, os.ModePerm)
	}

	var err error
	BuyOrder, err = grocksdb.OpenDb(opts, buyOrderPath)
	if err != nil {
		panic(err)
	}
	SellOrder, err = grocksdb.OpenDb(opts, sellOrderPath)
	if err != nil {
		panic(err)
	}
}
