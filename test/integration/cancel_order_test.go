package engine_test

import (
	"fmt"
	"net/http"
	"testing"
	"trading-bsx/cmd/api/server"
	"trading-bsx/internal/trade"
	"trading-bsx/pkg/db/models"
	"trading-bsx/pkg/testutil"

	"github.com/stretchr/testify/assert"
)

func Test_CancelOrder(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()
	client := testutil.NewClient(s)
	client.SetUser(1)

	resp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  100.0,
		Volume: 10,
	})
	orderId := resp.OpenOrderKey
	requireValidOrderKey(t, orderId)

	res := client.Request(&testutil.RequestOption{
		Method: http.MethodDelete,
		URL:    fmt.Sprintf("/orders/%s", orderId),
	})
	assert.Equal(t, http.StatusOK, res.Code)

	orders := getOrders(t, client)
	assert.Empty(t, orders)
}

func Test_CancelOrder_MustNotMatch_NewOrder(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()
	client := testutil.NewClient(s)
	client.SetUser(1)

	resp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  100.0,
		Volume: 6,
	})
	orderId := resp.OpenOrderKey

	res := client.Request(&testutil.RequestOption{
		Method: http.MethodDelete,
		URL:    fmt.Sprintf("/orders/%s", orderId),
	})
	assert.Equal(t, http.StatusOK, res.Code)

	client.SetUser(2)
	resp = placeOrder(t, client, trade.CreateOrder{
		Type:   models.BUY,
		Price:  101.0,
		Volume: 4,
	})
	assert.Equal(t, uint64(0), resp.FilledVolume)
	assert.Equal(t, uint64(4), resp.RemainingVolume)
	requireValidOrderKey(t, resp.OpenOrderKey)
}

func Test_CancelOrder_RemovesRemainingVolumeOfPartialOrder(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(1)
	sellResp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  100.0,
		Volume: 10,
	})

	client.SetUser(2)
	buyResp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.BUY,
		Price:  100.0,
		Volume: 4,
	})
	assert.Equal(t, uint64(4), buyResp.FilledVolume)
	assert.Equal(t, uint64(0), buyResp.RemainingVolume)

	client.SetUser(1)
	res := client.Request(&testutil.RequestOption{
		Method: http.MethodDelete,
		URL:    fmt.Sprintf("/orders/%s", sellResp.OpenOrderKey),
	})
	assert.Equal(t, http.StatusOK, res.Code)

	orders := getOrders(t, client)
	assert.Empty(t, orders)
}
