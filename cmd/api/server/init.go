package server

import (
	"os"
	"trading-bsx/internal/middleware"
	"trading-bsx/internal/trade"
	"trading-bsx/pkg/db/rocksdb"
	"trading-bsx/pkg/utils"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func New() *echo.Echo {
	if os.Getenv("ENV") == "test" {
		godotenv.Load("../../.env")
		zerolog.SetGlobalLevel(zerolog.Disabled)
	} else {
		godotenv.Load()
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out: os.Stdout,
	})

	rocksdb.Init()

	e := echo.New()
	e.HTTPErrorHandler = utils.HttpErrorHandler
	e.Validator = utils.NewValidator()
	e.Use(middleware.VerifyUser)

	order := e.Group("/orders")
	order.GET("", trade.GetOrders)
	order.POST("", trade.PlaceOrder)
	order.DELETE("/:order_id", trade.CancelOrder)

	return e
}
