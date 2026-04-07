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
	Type  models.OrderType `json:"type" validate:"required,oneof=BUY SELL"`
	Price float64          `json:"price" validate:"required,gt=0"`
	// Good till time, in milliseconds
	GTT *uint64 `json:"gtt,omitempty" validate:"omitempty,gt=0"`
}

var mutex = sync.Mutex{}

func PlaceOrder(c echo.Context) error {
	body := CreateOrder{}
	if err := utils.BindNValidate(c, &body); err != nil {
		fmt.Println(err)
		return err
	}

	var book *grocksdb.DB
	var opponentBook *grocksdb.DB
	var matchOrder *models.Order
	var matchOrderKey []byte

	order := models.Order{
		UserId:    c.Get("userId").(uint64),
		Type:      body.Type,
		Price:     body.Price,
		Timestamp: uint64(time.Now().UnixNano()),
		ExpiredAt: nil,
	}
	if body.GTT != nil {
		tmp := *body.GTT*uint64(time.Millisecond) + order.Timestamp
		order.ExpiredAt = &tmp
	}

	mutex.Lock()
	defer mutex.Unlock()

	if order.Type == models.BUY {
		book = rocksdb.BuyOrder
		opponentBook = rocksdb.SellOrder
		matchOrderKey, matchOrder = getMatchSellOrder(&order)
	} else {
		book = rocksdb.SellOrder
		opponentBook = rocksdb.BuyOrder
		matchOrderKey, matchOrder = getMatchBuyOrder(&order)
	}

	log.Info().Interface("order", order).Msg("Place order")
	log.Info().Interface("matchOrder", matchOrder).Msg("Match order")

	wo := grocksdb.NewDefaultWriteOptions()
	defer wo.Destroy()

	reqCtx := c.Request().Context()
	if matchOrder != nil {
		if err := opponentBook.Delete(wo, matchOrderKey); err != nil {
			return err
		}
		deleteMirroredOrder(reqCtx, matchOrder.Key)
		return c.JSON(http.StatusOK, matchOrder)
	}

	orderKey, orderValue := order.ToKVBytes()
	if err := book.Put(wo, orderKey, orderValue); err != nil {
		return err
	}
	orderID, err := createOrderRecord(reqCtx, order)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, orderID)
}

func getMatchBuyOrder(order *models.Order) ([]byte, *models.Order) {
	// Sell -> Get biggest buy order -> Seek from the last item in the list
	ro := grocksdb.NewDefaultReadOptions()
	defer ro.Destroy()
	it := rocksdb.BuyOrder.NewIterator(ro)
	defer it.Close()

	it.SeekToLast()
	for it.Valid() {
		k, v := it.Key().Data(), it.Value().Data()
		matchOrder := models.Order{
			Type: models.BUY,
		}
		matchOrder.ParseKV(k, v)
		if matchOrder.UserId == order.UserId {
			it.Prev()
			continue
		}
		if matchOrder.ExpiredAt != nil && *matchOrder.ExpiredAt > 0 {
			if uint64(time.Now().UnixNano()) > *matchOrder.ExpiredAt {
				wo := grocksdb.NewDefaultWriteOptions()
				defer wo.Destroy()
				if err := rocksdb.BuyOrder.Delete(wo, k); err != nil {
					fmt.Println(err)
				}
				it.Prev()
				continue
			}
		}
		if matchOrder.Price >= order.Price {
			return k, &matchOrder
		}

		// The biggest buy order is smaller than the current order, so no need to continue
		return nil, nil
	}
	return nil, nil
}

func getMatchSellOrder(order *models.Order) ([]byte, *models.Order) {
	// Buy -> Get smallest sell order -> Seek from the first item in the list
	ro := grocksdb.NewDefaultReadOptions()
	defer ro.Destroy()
	it := rocksdb.SellOrder.NewIterator(ro)
	defer it.Close()

	it.SeekToFirst()
	for it.Valid() {
		k, v := it.Key().Data(), it.Value().Data()
		matchOrder := models.Order{
			Type: models.SELL,
		}
		matchOrder.ParseKV(k, v)
		if matchOrder.UserId == order.UserId {
			it.Next()
			continue
		}
		if matchOrder.ExpiredAt != nil && *matchOrder.ExpiredAt > 0 {
			if uint64(time.Now().UnixNano()) > *matchOrder.ExpiredAt {
				wo := grocksdb.NewDefaultWriteOptions()
				defer wo.Destroy()
				if err := rocksdb.SellOrder.Delete(wo, k); err != nil {
					fmt.Println(err)
				}
				it.Next()
				continue
			}
		}
		if matchOrder.Price <= order.Price {
			return k, &matchOrder
		}

		// The smallest sell order is bigger than the current order, so no need to continue
		return nil, nil
	}
	return nil, nil
}
