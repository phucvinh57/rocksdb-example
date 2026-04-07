package trade

import (
	"context"
	"encoding/base32"
	"net/http"
	"sort"
	"time"
	"trading-bsx/pkg/config"
	"trading-bsx/pkg/db/models"
	"trading-bsx/pkg/db/mongodb"
	"trading-bsx/pkg/db/rocksdb"

	"github.com/labstack/echo/v4"
	"github.com/linxGnu/grocksdb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func createOrderRecord(ctx context.Context, order models.Order) (string, error) {
	if !config.MongoEnabled() {
		return order.Key, nil
	}

	result, err := mongodb.Order.InsertOne(ctx, order)
	if err != nil {
		return "", err
	}

	return result.InsertedID.(primitive.ObjectID).Hex(), nil
}

func deleteMirroredOrder(ctx context.Context, key string) {
	if !config.MongoEnabled() {
		return
	}

	mongodb.Order.DeleteOne(ctx, bson.M{"key": key})
}

func getUserOrders(ctx context.Context, userId uint64) ([]models.Order, error) {
	if config.MongoEnabled() {
		return getUserOrdersFromMongo(ctx, userId)
	}

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

func getUserOrdersFromMongo(ctx context.Context, userId uint64) ([]models.Order, error) {
	orders := make([]models.Order, 0)
	ts := uint64(time.Now().UnixNano())
	filter := bson.M{
		"$and": []bson.M{
			{"user_id": userId},
			{
				"$or": []bson.M{
					{"expired_at": bson.M{"$gte": ts}},
					{"expired_at": 0},
					{"expired_at": bson.M{"$exists": false}},
				},
			},
		},
	}
	cursor, err := mongodb.Order.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &orders); err != nil {
		return nil, err
	}

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
		if order.ExpiredAt != nil && *order.ExpiredAt > 0 && now > *order.ExpiredAt {
			continue
		}

		orders = append(orders, order)
	}

	return orders, it.Err()
}

func cancelOrderRecord(ctx context.Context, userId uint64, orderID string) (*models.Order, *grocksdb.DB, []byte, error) {
	if config.MongoEnabled() {
		return cancelMongoOrder(ctx, userId, orderID)
	}

	return cancelRocksOrder(userId, orderID)
}

func cancelMongoOrder(ctx context.Context, userId uint64, orderID string) (*models.Order, *grocksdb.DB, []byte, error) {
	objectID, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		return nil, nil, nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid order id")
	}

	order := models.Order{}
	if err := mongodb.Order.FindOneAndDelete(ctx, bson.M{
		"_id":     objectID,
		"user_id": userId,
	}).Decode(&order); err != nil {
		return nil, nil, nil, err
	}

	orderKey, err := base32.StdEncoding.DecodeString(order.Key)
	if err != nil {
		return nil, nil, nil, err
	}

	return &order, getBookByType(order.Type), orderKey, nil
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
		if order.ExpiredAt != nil && *order.ExpiredAt > 0 && uint64(time.Now().UnixNano()) > *order.ExpiredAt {
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
