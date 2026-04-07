package engine_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"trading-bsx/cmd/api/server"
	"trading-bsx/internal/trade"
	"trading-bsx/pkg/db/models"
	"trading-bsx/pkg/testutil"

	"github.com/stretchr/testify/assert"
)

func Test_InMemory_PlaceAndGetOrders(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("ROCKSDB_IN_MEMORY", "true")

	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(42)

	resp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.BUY,
		Price:  123.45,
		Volume: 7,
	})
	assert.Equal(t, uint64(7), resp.RemainingVolume)
	requireValidOrderKey(t, resp.OpenOrderKey)

	orders := getOrders(t, client)
	assert.Len(t, orders, 1)
	assert.Equal(t, resp.OpenOrderKey, orders[0].Key)
	assert.Equal(t, models.BUY, orders[0].Type)
	assert.Equal(t, uint64(7), orders[0].Volume)
}

func Test_InMemory_CancelOrder(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("ROCKSDB_IN_MEMORY", "true")

	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(7)

	resp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  99.9,
		Volume: 3,
	})
	res := client.Request(&testutil.RequestOption{
		Method: http.MethodDelete,
		URL:    fmt.Sprintf("/orders/%s", resp.OpenOrderKey),
	})
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, resp.OpenOrderKey, strings.TrimSpace(res.Body.String()))

	orders := getOrders(t, client)
	assert.Empty(t, orders)
}
