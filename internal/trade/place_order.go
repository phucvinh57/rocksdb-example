package trade

import (
	"fmt"
	"net/http"
	"sync"
	"time"
	"trading-bsx/pkg/db/models"
	"trading-bsx/pkg/db/rocksdb"
	"trading-bsx/pkg/utils"

	"github.com/labstack/echo/v4"
	"github.com/linxGnu/grocksdb"
	"github.com/rs/zerolog/log"
)

type CreateOrder struct {
	Type   models.OrderType `json:"type" validate:"required,oneof=BUY SELL"`
	Price  float64          `json:"price" validate:"required,gt=0"`
	Volume uint64           `json:"volume" validate:"required,gt=0"`
	// Good till time, in milliseconds
	GTT *uint64 `json:"gtt,omitempty" validate:"omitempty,gt=0"`
}

type TradeFill struct {
	OrderKey  string  `json:"orderKey"`
	UserId    uint64  `json:"userId"`
	Price     float64 `json:"price"`
	Volume    uint64  `json:"volume"`
	Timestamp uint64  `json:"timestamp"`
}

type PlaceOrderResponse struct {
	Type            models.OrderType `json:"type"`
	Price           float64          `json:"price"`
	RequestedVolume uint64           `json:"requestedVolume"`
	FilledVolume    uint64           `json:"filledVolume"`
	RemainingVolume uint64           `json:"remainingVolume"`
	Fills           []TradeFill      `json:"fills"`
	OpenOrderKey    string           `json:"openOrderKey,omitempty"`
}

var mutex = sync.Mutex{}

func PlaceOrder(c echo.Context) error {
	body := CreateOrder{}
	if err := utils.BindNValidate(c, &body); err != nil {
		fmt.Println(err)
		return err
	}

	order := models.Order{
		UserId:    c.Get("userId").(uint64),
		Type:      body.Type,
		Price:     body.Price,
		Volume:    body.Volume,
		Timestamp: uint64(time.Now().UnixNano()),
		ExpiredAt: nil,
	}
	if body.GTT != nil {
		tmp := *body.GTT*uint64(time.Millisecond) + order.Timestamp
		order.ExpiredAt = &tmp
	}

	response, err := placeOrder(order)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, response)
}

func placeOrder(order models.Order) (*PlaceOrderResponse, error) {
	var book *grocksdb.DB
	var opponentBook *grocksdb.DB

	requestedVolume := order.Volume

	mutex.Lock()
	defer mutex.Unlock()

	if order.Type == models.BUY {
		book = rocksdb.BuyOrder
		opponentBook = rocksdb.SellOrder
	} else {
		book = rocksdb.SellOrder
		opponentBook = rocksdb.BuyOrder
	}

	fills, err := matchOrder(&order, opponentBook)
	if err != nil {
		return nil, err
	}

	log.Info().Interface("order", order).Msg("Place order")
	log.Info().Interface("fills", fills).Msg("Matched fills")

	wo := grocksdb.NewDefaultWriteOptions()
	defer wo.Destroy()

	response := &PlaceOrderResponse{
		Type:            order.Type,
		Price:           order.Price,
		RequestedVolume: requestedVolume,
		FilledVolume:    requestedVolume - order.Volume,
		RemainingVolume: order.Volume,
		Fills:           fills,
	}

	if order.Volume > 0 {
		orderKey, orderValue := order.ToKVBytes()
		if err := book.Put(wo, orderKey, orderValue); err != nil {
			return nil, err
		}
		response.OpenOrderKey = order.Key
	}

	return response, nil
}

func matchOrder(order *models.Order, opponentBook *grocksdb.DB) ([]TradeFill, error) {
	ro := grocksdb.NewDefaultReadOptions()
	defer ro.Destroy()
	it := opponentBook.NewIterator(ro)
	defer it.Close()

	wo := grocksdb.NewDefaultWriteOptions()
	defer wo.Destroy()

	matchType := models.SELL
	step := func(it *grocksdb.Iterator) {
		it.Next()
	}
	if order.Type == models.SELL {
		matchType = models.BUY
		step = func(it *grocksdb.Iterator) {
			it.Prev()
		}
		it.SeekToLast()
	} else {
		it.SeekToFirst()
	}

	fills := make([]TradeFill, 0)
	for it.Valid() && order.Volume > 0 {
		key := it.Key()
		value := it.Value()
		keyBytes := append([]byte(nil), key.Data()...)
		valueBytes := append([]byte(nil), value.Data()...)
		key.Free()
		value.Free()

		matchOrder := models.Order{Type: matchType}
		matchOrder.ParseKV(keyBytes, valueBytes)
		if matchOrder.UserId == order.UserId {
			step(it)
			continue
		}
		if isOrderExpired(matchOrder, uint64(time.Now().UnixNano())) {
			if err := opponentBook.Delete(wo, keyBytes); err != nil {
				return nil, err
			}
			step(it)
			continue
		}
		if !pricesMatch(order, matchOrder) {
			break
		}

		fillVolume := order.Volume
		if matchOrder.Volume < fillVolume {
			fillVolume = matchOrder.Volume
		}
		fills = append(fills, TradeFill{
			OrderKey:  matchOrder.Key,
			UserId:    matchOrder.UserId,
			Price:     matchOrder.Price,
			Volume:    fillVolume,
			Timestamp: matchOrder.Timestamp,
		})
		order.Volume -= fillVolume

		if fillVolume == matchOrder.Volume {
			if err := opponentBook.Delete(wo, keyBytes); err != nil {
				return nil, err
			}
			step(it)
			continue
		}

		matchOrder.Volume -= fillVolume
		_, updatedValue := matchOrder.ToKVBytes()
		if err := opponentBook.Put(wo, keyBytes, updatedValue); err != nil {
			return nil, err
		}
		break
	}

	if err := it.Err(); err != nil {
		return nil, err
	}

	return fills, nil
}

func pricesMatch(incoming *models.Order, resting models.Order) bool {
	if incoming.Type == models.BUY {
		return resting.Price <= incoming.Price
	}

	return resting.Price >= incoming.Price
}
