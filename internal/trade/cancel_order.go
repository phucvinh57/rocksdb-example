package trade

import (
	"fmt"
	"net/http"
	"trading-bsx/pkg/utils"

	"github.com/labstack/echo/v4"
	"github.com/linxGnu/grocksdb"
	"github.com/rs/zerolog/log"
)

type DeleteOrder struct {
	OrderId string `param:"order_id" validate:"required"`
}

func CancelOrder(c echo.Context) error {
	req := DeleteOrder{}
	if err := utils.BindNValidate(c, &req); err != nil {
		fmt.Println(err)
		return err
	}
	userId := c.Get("userId").(uint64)

	order, book, orderKey, err := cancelOrderRecord(c.Request().Context(), userId, req.OrderId)
	if err != nil {
		return err
	}

	mutex.Lock()
	defer mutex.Unlock()
	wo := grocksdb.NewDefaultWriteOptions()
	defer wo.Destroy()

	if err := book.Delete(wo, orderKey); err != nil {
		return err
	}

	log.Info().Interface("order", order).Msg("Cancel order")
	return c.String(http.StatusOK, req.OrderId)
}
