package engine_test

import (
	"testing"
	"trading-bsx/cmd/api/server"
	"trading-bsx/internal/trade"
	"trading-bsx/pkg/db/models"
	"trading-bsx/pkg/testutil"

	"github.com/stretchr/testify/assert"
)

func Test_PlaceOrders(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	var prices = []float64{100.5, 110.2, 120.3, 130.4, 140.5, 150.6, 160.7, 170.8, 180.9, 190.0}

	for _, price := range prices {
		for j := 1; j <= 2; j++ {
			client.SetUser(uint64(j))
			resp := placeOrder(t, client, trade.CreateOrder{
				Type:   models.BUY,
				Price:  price,
				Volume: 2,
			})
			assert.Equal(t, uint64(0), resp.FilledVolume)
			assert.Equal(t, uint64(2), resp.RemainingVolume)
			requireValidOrderKey(t, resp.OpenOrderKey)
		}
	}

	client.SetUser(1)
	orders := getOrders(t, client)
	assert.Len(t, orders, len(prices))
	for _, order := range orders {
		assert.Equal(t, uint64(2), order.Volume)
	}
}
