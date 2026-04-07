package trade

import (
	"encoding/base32"
	"net/http"
	"sort"
	"time"
	"trading-bsx/pkg/db/models"
	"trading-bsx/pkg/db/rocksdb"

	"github.com/labstack/echo/v4"
	"github.com/linxGnu/grocksdb"
)

func getUserOrders(userId uint64) ([]models.Order, error) {
	mutex.Lock()
	defer mutex.Unlock()

	now := uint64(time.Now().UnixNano())

	buyOrders, err := getUserOrdersFromBook(rocksdb.BuyOrder, models.BUY, userId, now)
	if err != nil {
		return nil, err
	}

	sellOrders, err := getUserOrdersFromBook(rocksdb.SellOrder, models.SELL, userId, now)
	if err != nil {
		return nil, err
	}
	orders := make([]models.Order, 0, len(buyOrders)+len(sellOrders))
	orders = append(orders, buyOrders...)
	orders = append(orders, sellOrders...)

	sort.Slice(orders, func(i, j int) bool {
		return orders[i].Timestamp < orders[j].Timestamp
	})

	return orders, nil
}

func getUserOrdersFromBook(book *grocksdb.DB, orderType models.OrderType, userId uint64, now uint64) ([]models.Order, error) {
	ro := grocksdb.NewDefaultReadOptions()
	defer ro.Destroy()
	ro.SetFillCache(false)

	it := book.NewIterator(ro)
	defer it.Close()

	orders := make([]models.Order, 0)
	for it.SeekToFirst(); it.Valid(); it.Next() {
		key := it.Key()
		value := it.Value()

		keyBytes := append([]byte(nil), key.Data()...)
		valueBytes := append([]byte(nil), value.Data()...)
		key.Free()
		value.Free()

		order := models.Order{Type: orderType}
		order.ParseKV(keyBytes, valueBytes)

		if order.UserId != userId {
			continue
		}
		if isOrderExpired(order, now) {
			continue
		}

		orders = append(orders, order)
	}

	return orders, it.Err()
}

func cancelOrderRecord(userId uint64, orderID string) (*models.Order, *grocksdb.DB, []byte, error) {
	return cancelRocksOrder(userId, orderID)
}

func cancelRocksOrder(userId uint64, orderID string) (*models.Order, *grocksdb.DB, []byte, error) {
	orderKey, err := base32.StdEncoding.DecodeString(orderID)
	if err != nil {
		return nil, nil, nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid order id")
	}

	ro := grocksdb.NewDefaultReadOptions()
	defer ro.Destroy()

	for _, candidate := range []struct {
		book      *grocksdb.DB
		orderType models.OrderType
	}{
		{book: rocksdb.BuyOrder, orderType: models.BUY},
		{book: rocksdb.SellOrder, orderType: models.SELL},
	} {
		value, err := candidate.book.GetBytes(ro, orderKey)
		if err != nil {
			return nil, nil, nil, err
		}
		if value == nil {
			continue
		}

		order := models.Order{Type: candidate.orderType}
		order.ParseKV(orderKey, value)
		if order.UserId != userId {
			return nil, nil, nil, echo.NewHTTPError(http.StatusNotFound, "Resource not found")
		}
		if isOrderExpired(order, uint64(time.Now().UnixNano())) {
			return nil, nil, nil, echo.NewHTTPError(http.StatusNotFound, "Resource not found")
		}

		return &order, candidate.book, orderKey, nil
	}

	return nil, nil, nil, echo.NewHTTPError(http.StatusNotFound, "Resource not found")
}

func getBookByType(orderType models.OrderType) *grocksdb.DB {
	if orderType == models.BUY {
		return rocksdb.BuyOrder
	}

	return rocksdb.SellOrder
}

func isOrderExpired(order models.Order, now uint64) bool {
	return order.ExpiredAt != nil && *order.ExpiredAt > 0 && now > *order.ExpiredAt
}
