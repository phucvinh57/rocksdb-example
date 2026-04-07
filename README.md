# BSX trading challenge

Challenge description: <https://gist.github.com/bsx-engineering/b1f9b4d6f2fcd96e065953584c113b8c>

To be summarized, there are 3 APIs to be implemented:

- `GET /orders`: Get user's orders. Returned result must not include expired & matched orders
- `POST /orders`: Place a buy/sell order. Return a match order if exists. 
- `DELETE /orders/:id`: Cancel an order.

## Design

### Pick a data storage for order book

Our matching rule:
- If a user place a sell order, the system will try to match with the highest buy order.
- If a user place a buy order, the system will try to match with the lowest sell order.
- If there are multiple orders with the same price, earlier order will be matched first.

Our order book should have these characteristics:
- Extremely fast insertion, deletion, and lookup.
- Easy to find the highest buy order and the lowest sell order.
- Durability: There must be no data loss when an accidence happens (power failure, system crash ...).

An ordered key-value store (OKVS) is a good choice for this problem. There are many options like Redis, DynamoDB, RocksDB, LevelDB, etc. Following [CAP theorem](https://en.wikipedia.org/wiki/CAP_theorem), we must pick two of three properties: **Consistency**, **Availability**, and **Partition tolerance**. Due to our order book characteristics, Consistency & Availibility are more important than Partition tolerance. 

Technical decisions:
- Modern OKVSs like Redis & DynamoDB are hosted services, stay outside of the internet, and have network latency. They sacrifice consistency and choose availability & partition tolerance => Not suitable
- RocksDB & LevelDB satisfy our requirements. But LevelDB's key & value are strings, which are not suitable for our use case. In contrast, RocksDB allows us to store keys & values as bytes  => RocksDB is chosen.

### Data structure

We will have 2 RocksDB instances: `BuyOrder` & `SellOrder`, which store buy & sell orders respectively. Each order is stored as a key-value pair.

- Key has 32 bytes: 16 bytes for price, 8 bytes for timestamp & 8 bytes for user ID. 
  - We store price by multiplying by 10^18 (Wei unit) to keep precision. A product of a `float64` and `10^18` must be stored with `64 + log2(10^18) ~= 123` bits => use 128 bits <=> 16 bytes.
  - Timestamp is Unix time in nanoseconds. It's a 64-bit integer, which is 8 bytes.
  - User ID is a 64-bit integer, which is 8 bytes.
- Value: We don't need to restrict the value's size. It can store JSON, number, string, ... in bytes. In our case, we store the order's `good till time` (gtt).

Records in RocksDB are sorted by keys, in byte order. To get the highest buy order, we just pick the last record in `BuyOrder`. To get the lowest sell order, we just pick the first record in `SellOrder`. If the record has same user ID with new order, we can move to the next record. In case the record is expired, we can delete immediately.

### Data replication

Our key & value design is only optimized for order matching, not for the feature get user's orders. In this feature, user ID is used as a key to get all orders of a user. If we store all orders in a single RocksDB instance, we must scan all records to get user's orders. This is not efficient.

To solve this problem, we replicate order data into another database. In this challenge, MongoDB is chosen. We store user's orders in MongoDB with an index on user ID.

There are 2 ways to replicate data:

- Synchronous replication: After writing to RocksDB, we write to MongoDB. This way is slow and not safe because if writing to MongoDB fails, we must rollback the write to RocksDB.
- Asynchronous replication: We write to RocksDB first, then push the write to a message queue. Kafka is a good choice for this. A consumer will consume message in batches and write to MongoDB. This way is faster and safer.

Due to the time limit, we choose the synchronous replication.

## Implementation

### Prerequisites

- Install C libraries: `sudo apt install librocksdb-dev libsnappy-dev libz-dev liblz4-dev libzstd-dev`
- Install dependencies: `go mod download`

### Storage modes

By default, the app uses RocksDB for matching and MongoDB for the user-order index.

- `MONGODB_ENABLED=true`: keep the current MongoDB-backed order index
- `MONGODB_ENABLED=false` or `IGNORE_MONGODB=true`: disable MongoDB entirely
- `ROCKSDB_IN_MEMORY=true`: run RocksDB with an in-memory environment instead of on-disk files

Example: run fully in RAM without MongoDB

```bash
MONGODB_ENABLED=false ROCKSDB_IN_MEMORY=true go test ./test/integration -run RocksOnly
```

When MongoDB is disabled:

- `POST /orders` returns the base32-encoded RocksDB key instead of a MongoDB ObjectID
- `DELETE /orders/:id` accepts that returned key
- `GET /orders` still works by scanning both RocksDB books and filtering by user ID

### Testing

There are some provided test cases in `./test/integration`:

- Rapid place orders & get orders. Number of orders of a user must equal to the number of orders placed by him.
- Match buy orders: Place multiple buy orders, then place sell orders. Some buy orders match, some don't. Finally, check size of order book.
- Match sell orders: Similar to match buy orders.
- Cancel orders: Place orders, then cancel them. New order must not match canceled order. Finally, check size of order book.
- Expire orders: Place orders, then wait until they expire. New order must not match expired order. Finally, check size of order book.

Run `make test` to run all test cases.

### Benchmark

Random 200 users, place orders rapidly, each order has a random price in range [100, 200]. 

There are 2 benchmarks:
- All users places only buy orders.
- Users places random buy & sell orders.

Run `make benchmark` to run the benchmarks.

Result:

```bash
> go test -bench=. -benchmem -benchtime=10s ./test/bench

goos: linux
goarch: amd64
pkg: trading-bsx/test/bench
cpu: Intel(R) Core(TM) i5-7200U CPU @ 2.50GHz
Benchmark_PlaceOnlyOneOrderType-4                  11304           1062627 ns/op           13044 B/op        109 allocs/op
Benchmark_PlaceRandomBuyNSellOrders-4               6254           2256801 ns/op           13356 B/op        119 allocs/op
PASS
ok      trading-bsx/test/bench  33.654s
```

In the first benchmark, each order is placed in `1.06ms`. In the second benchmark, each order is placed in `2.25ms`.

## Future development

- Use Kafka for asynchronous replication.
- Add quantity (volume) attribute to orders.
