.PHONY: build test dev clean bench benchmark

dev:
	air -c .air.toml
build:
	CGO_CFLAGS="-I/usr/include/rocksdb" \
	CGO_LDFLAGS="-L/usr/include/rocksdb -lrocksdb -lstdc++ -lm -lz -lsnappy -llz4 -lzstd" \
	go build -o main ./cmd/api
test:
	go test -v ./...
bench:
	go test -bench=. -benchmem -benchtime=10s ./test/bench
benchmark: bench
clean:
	rm -f main
	rm -rf tmp **/*/rocksdb_data rocksdb_data
