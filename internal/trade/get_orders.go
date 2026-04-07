package trade

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func GetOrders(c echo.Context) error {
	userId := c.Get("userId").(uint64)
	orders, err := getUserOrders(userId)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, orders)
}
