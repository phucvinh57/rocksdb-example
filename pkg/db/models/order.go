package models

import (
	"encoding/base32"
	"encoding/binary"
	"math/big"
)

type OrderType string

const (
	BUY  OrderType = "BUY"
	SELL OrderType = "SELL"
)

type Order struct {
	UserId    uint64    `json:"userId"`
	Type      OrderType `json:"type"`
	Price     float64   `json:"price"`
	Volume    uint64    `json:"volume"`
	ExpiredAt *uint64   `json:"expiredAt,omitempty"`
	Timestamp uint64    `json:"timestamp,omitempty"`
	Key       string    `json:"key,omitempty"`
}

func (order *Order) ParseKV(key []byte, value []byte) {
	price := new(big.Int).SetBytes(key[:16])
	priceFloat := big.NewFloat(0).SetInt(price)
	priceFloat.Quo(priceFloat, big.NewFloat(WEI18))
	order.Price, _ = priceFloat.Float64()

	ts := binary.BigEndian.Uint64(key[16:24])
	if order.Type == BUY {
		ts = ^ts
	}
	order.Timestamp = ts
	order.UserId = binary.BigEndian.Uint64(key[24:32])

	if len(value) >= 8 {
		order.Volume = binary.BigEndian.Uint64(value[:8])
	}
	if len(value) >= 16 {
		exp := binary.BigEndian.Uint64(value[8:16])
		order.ExpiredAt = &exp
	}
	if order.ExpiredAt != nil && *order.ExpiredAt == 0 {
		order.ExpiredAt = nil
	}

	order.Key = base32.StdEncoding.EncodeToString(key)
}

func (order *Order) ToKVBytes() ([]byte, []byte) {
	// 16 bytes for price, 8 bytes for timestamp, 8 bytes for user ID
	key := make([]byte, 32)

	rawPrice := big.NewFloat(order.Price)
	priceInt := big.NewInt(0)
	rawPrice.Mul(rawPrice, big.NewFloat(WEI18)).Int(priceInt)
	copy(key[16-len(priceInt.Bytes()):], priceInt.Bytes())

	ts := order.Timestamp
	if order.Type == BUY {
		ts = ^ts
	}
	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, ts)

	copy(key[16:24], timestampBytes)

	userIdBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(userIdBytes, order.UserId)
	copy(key[24:32], userIdBytes)

	value := make([]byte, 16)
	binary.BigEndian.PutUint64(value[:8], order.Volume)
	if order.ExpiredAt != nil {
		binary.BigEndian.PutUint64(value[8:16], *order.ExpiredAt)
	}
	order.Key = base32.StdEncoding.EncodeToString(key)
	return key, value
}

const WEI18 = 1e18
