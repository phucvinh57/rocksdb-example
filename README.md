# Trading example with RocksDB

## Description

You operate a marketplace for buying & selling used textbooks. For a given textbook, e.g. "Theory of Cryptography," there are people who want to buy this textbook and people who want to sell.
-  Offers to BUY: $100, $100, $99, $99, $97, $90
-  Offers to SELL: $109, $110, $110, $114, $115, $119

A match occurs when two people agree on a price. Some new offers do not match. These offers should be added to the active set of offers. For example:

Tim offers to SELL at $150. No one is willing to buy at that price so we save the offer.
-  Offers to SELL:: $109, $110, $110, $114, $115, $119, $150

When matching we want to give the customer the "best price". Example matches:

If Jane offers to BUY at $120, she will match and buy a book for $109 (the lowest offer to sell is the best price). The sell offers should be updated to reflect the match
- Offers to SELL: $110, $110, $114, $115, $119, $150

If Connie offers to SELL at $99 she will match and sell her book for $100 (the highest offer to buy is the best price). The buy offers should be updated to reflect the match
- Offers to BUY: $100, $99, $99, $97, $90

**Task**: Write the program to allow user to submit buy and sell order. The basic version of this system will need to:
- Accept incoming offers to buy & sell
    - Output if the price matches
- Keep an updated lists of buys & sells


### Time Triggers
Some customers would like their offers to expire. When orders are placed allow customers to specify an optional GTT (Good Til Time). If the offer is not matched before the GTT, the offer is deleted from the order book. Customer A could offer to buy at $90 with a GTT of one hour from now. If Customer B offers to sell at $90 before one hour has passed the order should be matched. If Customer B offers to sell at $90 2 hours later, the old order will be gone.


### Customer ID
To really match a buyer and a seller you would need to keep track of which offers belong to which customer. Extend your implementation to track the customer id. The customer who placed the order first should be prefered. 
**A customer should not be able to sell a book to themselves.**

Extend your implementation to allow a user to cancel their order. If a user cancels their order, the order should be removed from the order book.

Add a function to allow a user to query all their existing orders. This should return a list of all their orders that have not been matched or canceled.

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
- Value has 16 bytes:
  - 8 bytes for remaining `volume`
  - 8 bytes for `expiredAt` in Unix nanoseconds, where `0` means no expiry

Records in RocksDB are sorted by keys, in byte order. To get the highest buy order, we pick the last record in `BuyOrder`. To get the lowest sell order, we pick the first record in `SellOrder`. Matching walks the opposite book in price-time order, skips self-matches, removes expired records lazily, and can consume multiple resting orders until the incoming order is fully filled or no more matchable orders remain.

### Reading user orders

Our key design is optimized for matching, not for querying by user ID. Since the project now uses only RocksDB, `GET /orders` scans both books, filters by `userId`, and skips expired records in memory. This keeps the storage model simple and makes the whole service self-contained, with the tradeoff that user-order lookups are linear in the current book size.

## Implementation

### Prerequisites

- Install C libraries: `sudo apt install librocksdb-dev libsnappy-dev libz-dev liblz4-dev libzstd-dev`
- Install dependencies: `go mod download`

### Runtime configuration

The service only depends on RocksDB.

- `ROCKSDB_IN_MEMORY=true`: run RocksDB with an in-memory environment instead of on-disk files

Example: run fully in RAM

```bash
ROCKSDB_IN_MEMORY=true go test ./test/integration -run InMemory
```

`POST /orders` request body:

```json
{
  "type": "BUY",
  "price": 101.5,
  "volume": 10,
  "gtt": 1000
}
```

`POST /orders` response body:

```json
{
  "type": "BUY",
  "price": 101.5,
  "requestedVolume": 10,
  "filledVolume": 7,
  "remainingVolume": 3,
  "fills": [
    {
      "orderKey": "BASE32KEY...",
      "userId": 2,
      "price": 100.0,
      "volume": 7,
      "timestamp": 1710000000000000000
    }
  ],
  "openOrderKey": "BASE32KEY..."
}
```

`GET /orders` returns open orders with remaining `volume`, and `DELETE /orders/:id` accepts the `openOrderKey`.

### Testing

There are some provided test cases in `./test/integration`:

- Rapid place orders and fetch remaining open orders
- Full-fill and partial-fill matching on both sides of the book
- Multi-fill matching across several resting orders in price-time order
- Self-match skipping
- Canceling the remaining volume of an open order
- Expired orders being skipped and cleaned up during matching

Run `make test` to run all test cases.

### Benchmark

Random 200 users, place orders rapidly, each order has a random price in range [100, 200]. 

Current benchmark suite:
- On-disk RocksDB, users place random buy & sell orders
- In-memory RocksDB, users place random buy & sell orders

Run `make benchmark` to run the benchmarks.

Result on this machine:

- OS: Linux amd64
- CPU: 13th Gen Intel(R) Core(TM) i7-13800H
- Benchmarks were run separately:
  - `go test ./test/bench -bench '^Benchmark_PlaceRandomBuyNSellOrders$' -benchmem -benchtime=10s`
  - `go test ./test/bench -bench '^Benchmark_InMemoryPlaceRandomBuyNSellOrders$' -benchmem`

| Benchmark | ns/op | Time per tx | Approx tx/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| `Benchmark_PlaceRandomBuyNSellOrders` | 447,782 | 447.78 µs | 2,233 | 9,077 | 64 |
| `Benchmark_InMemoryPlaceRandomBuyNSellOrders` | 189,905 | 189.90 µs | 5,266 | 9,078 | 64 |

`Approx tx/s` is calculated as `1e9 / ns_op`.

## Future development

- Optimize the mixed-order matching path further.
- Add a trade history endpoint if executed fills need to be queried later.
