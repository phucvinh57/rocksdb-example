package engine_test

import (
	"testing"
	"trading-bsx/cmd/api/server"
	"trading-bsx/internal/trade"
	"trading-bsx/pkg/db/models"
	"trading-bsx/pkg/testutil"

	"github.com/stretchr/testify/assert"
)

func Test_MatchBuyOrder_FullFill(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(1)

	prices := []float64{100.5, 110.2, 120.3, 130.4, 140.5, 150.6, 160.7, 170.8, 180.9, 190.0}
	for _, price := range prices {
		resp := placeOrder(t, client, trade.CreateOrder{
			Type:   models.BUY,
			Price:  price,
			Volume: 1,
		})
		assert.Equal(t, uint64(1), resp.RemainingVolume)
	}

	matchPrices := []float64{190.0, 180.9}
	client.SetUser(2)
	for _, matchPrice := range matchPrices {
		resp := placeOrder(t, client, trade.CreateOrder{
			Type:   models.SELL,
			Price:  100.0,
			Volume: 1,
		})
		assert.Equal(t, uint64(1), resp.RequestedVolume)
		assert.Equal(t, uint64(1), resp.FilledVolume)
		assert.Equal(t, uint64(0), resp.RemainingVolume)
		assert.Len(t, resp.Fills, 1)
		assert.Equal(t, matchPrice, resp.Fills[0].Price)
		assert.Equal(t, uint64(1), resp.Fills[0].Volume)
		assert.Empty(t, resp.OpenOrderKey)
	}

	resp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  500.0,
		Volume: 1,
	})
	assert.Equal(t, uint64(0), resp.FilledVolume)
	assert.Equal(t, uint64(1), resp.RemainingVolume)
	requireValidOrderKey(t, resp.OpenOrderKey)

	client.SetUser(1)
	user1Orders := getOrders(t, client)
	client.SetUser(2)
	user2Orders := getOrders(t, client)
	assert.Equal(t, len(prices)-len(matchPrices)+1, len(user1Orders)+len(user2Orders))
}

func Test_MatchSellOrder_FullFill(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(1)

	prices := []float64{100.5, 110.2, 120.3, 130.4, 140.5, 150.6, 160.7, 170.8, 180.9, 190.0}
	for _, price := range prices {
		resp := placeOrder(t, client, trade.CreateOrder{
			Type:   models.SELL,
			Price:  price,
			Volume: 1,
		})
		assert.Equal(t, uint64(1), resp.RemainingVolume)
	}

	matchPrices := []float64{100.5, 110.2}
	client.SetUser(2)
	for _, matchPrice := range matchPrices {
		resp := placeOrder(t, client, trade.CreateOrder{
			Type:   models.BUY,
			Price:  140.0,
			Volume: 1,
		})
		assert.Equal(t, uint64(1), resp.RequestedVolume)
		assert.Equal(t, uint64(1), resp.FilledVolume)
		assert.Equal(t, uint64(0), resp.RemainingVolume)
		assert.Len(t, resp.Fills, 1)
		assert.Equal(t, matchPrice, resp.Fills[0].Price)
		assert.Equal(t, uint64(1), resp.Fills[0].Volume)
		assert.Empty(t, resp.OpenOrderKey)
	}

	resp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.BUY,
		Price:  50.0,
		Volume: 1,
	})
	assert.Equal(t, uint64(0), resp.FilledVolume)
	assert.Equal(t, uint64(1), resp.RemainingVolume)
	requireValidOrderKey(t, resp.OpenOrderKey)

	client.SetUser(1)
	user1Orders := getOrders(t, client)
	client.SetUser(2)
	user2Orders := getOrders(t, client)
	assert.Equal(t, len(prices)-len(matchPrices)+1, len(user1Orders)+len(user2Orders))
}

func Test_IncomingOrder_PartiallyFillsAndStoresRemainder(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(1)
	placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  100.0,
		Volume: 5,
	})

	client.SetUser(2)
	resp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.BUY,
		Price:  110.0,
		Volume: 8,
	})

	assert.Equal(t, uint64(5), resp.FilledVolume)
	assert.Equal(t, uint64(3), resp.RemainingVolume)
	assert.Equal(t, uint64(8), resp.RequestedVolume)
	assert.Len(t, resp.Fills, 1)
	assert.Equal(t, uint64(5), resp.Fills[0].Volume)
	requireValidOrderKey(t, resp.OpenOrderKey)

	orders := getOrders(t, client)
	assert.Len(t, orders, 1)
	assert.Equal(t, uint64(3), orders[0].Volume)
	assert.Equal(t, resp.OpenOrderKey, orders[0].Key)
}

func Test_RestingOrder_PartiallyFilled_RemainsOpen(t *testing.T) {
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
	assert.Equal(t, uint64(4), buyResp.RequestedVolume)
	assert.Len(t, buyResp.Fills, 1)
	assert.Equal(t, uint64(4), buyResp.Fills[0].Volume)

	client.SetUser(1)
	orders := getOrders(t, client)
	assert.Len(t, orders, 1)
	assert.Equal(t, sellResp.OpenOrderKey, orders[0].Key)
	assert.Equal(t, uint64(6), orders[0].Volume)
}

func Test_MultiFill_MatchesAcrossSeveralOrders(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(1)
	resp1 := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  100.0,
		Volume: 2,
	})

	client.SetUser(2)
	resp2 := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  101.0,
		Volume: 3,
	})

	client.SetUser(3)
	resp3 := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  102.0,
		Volume: 4,
	})

	client.SetUser(4)
	buyResp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.BUY,
		Price:  105.0,
		Volume: 6,
	})

	assert.Equal(t, uint64(6), buyResp.FilledVolume)
	assert.Equal(t, uint64(0), buyResp.RemainingVolume)
	assert.Equal(t, uint64(6), buyResp.RequestedVolume)
	assert.Len(t, buyResp.Fills, 3)
	assert.Equal(t, resp1.OpenOrderKey, buyResp.Fills[0].OrderKey)
	assert.Equal(t, resp2.OpenOrderKey, buyResp.Fills[1].OrderKey)
	assert.Equal(t, resp3.OpenOrderKey, buyResp.Fills[2].OrderKey)
	assert.Equal(t, uint64(2), buyResp.Fills[0].Volume)
	assert.Equal(t, uint64(3), buyResp.Fills[1].Volume)
	assert.Equal(t, uint64(1), buyResp.Fills[2].Volume)
	assert.Equal(t, 100.0, buyResp.Fills[0].Price)
	assert.Equal(t, 101.0, buyResp.Fills[1].Price)
	assert.Equal(t, 102.0, buyResp.Fills[2].Price)

	client.SetUser(3)
	orders := getOrders(t, client)
	assert.Len(t, orders, 1)
	assert.Equal(t, uint64(3), orders[0].Volume)
	assert.Equal(t, resp3.OpenOrderKey, orders[0].Key)
}

func Test_SelfMatch_IsSkipped(t *testing.T) {
	t.Setenv("ENV", "test")
	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(1)
	selfSell := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  99.0,
		Volume: 5,
	})

	client.SetUser(2)
	otherSell := placeOrder(t, client, trade.CreateOrder{
		Type:   models.SELL,
		Price:  100.0,
		Volume: 4,
	})

	client.SetUser(1)
	buyResp := placeOrder(t, client, trade.CreateOrder{
		Type:   models.BUY,
		Price:  100.0,
		Volume: 2,
	})

	assert.Equal(t, uint64(2), buyResp.FilledVolume)
	assert.Equal(t, uint64(0), buyResp.RemainingVolume)
	assert.Equal(t, uint64(2), buyResp.RequestedVolume)
	assert.Len(t, buyResp.Fills, 1)
	assert.Equal(t, uint64(2), buyResp.Fills[0].Volume)
	assert.Equal(t, uint64(2), buyResp.Fills[0].UserId)
	assert.Equal(t, otherSell.OpenOrderKey, buyResp.Fills[0].OrderKey)

	orders := getOrders(t, client)
	assert.Len(t, orders, 1)
	assert.Equal(t, selfSell.OpenOrderKey, orders[0].Key)
	assert.Equal(t, uint64(5), orders[0].Volume)

	client.SetUser(2)
	orders = getOrders(t, client)
	assert.Len(t, orders, 1)
	assert.Equal(t, otherSell.OpenOrderKey, orders[0].Key)
	assert.Equal(t, uint64(2), orders[0].Volume)
}
