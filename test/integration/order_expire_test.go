package engine_test

import (
	"testing"
	"time"
	"trading-bsx/cmd/api/server"
	"trading-bsx/internal/trade"
	"trading-bsx/pkg/db/models"
	"trading-bsx/pkg/testutil"

	"github.com/stretchr/testify/assert"
)

func Test_ExpiredOrder_ShouldNotMatch(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()
	client := testutil.NewClient(s)
	client.SetUser(1)
	var gtt uint64 = 10
	placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  100.0,
		Volume: 5,
		GTT:    &gtt,
	})

	client.SetUser(2)
	time.Sleep(time.Duration(gtt) * time.Millisecond)
	resp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.BUY,
		Price:  101.0,
		Volume: 4,
	})

	assert.Equal(t, uint64(0), resp.FilledVolume)
	assert.Equal(t, uint64(4), resp.RemainingVolume)
	requireValidOrderKey(t, resp.OpenOrderKey)

	client.SetUser(1)
	orders := getOrders(t, client)
	assert.Empty(t, orders)
}
