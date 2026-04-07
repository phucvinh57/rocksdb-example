package engine_test

import (
	"encoding/base32"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"trading-bsx/internal/trade"
	"trading-bsx/pkg/db/models"
	"trading-bsx/pkg/testutil"

	"github.com/stretchr/testify/require"
)

func placeOrder(t testing.TB, client *testutil.Client, body trade.CreateOrder) trade.PlaceOrderResponse {
	t.Helper()

	res := client.Request(&testutil.RequestOption{
		Method: http.MethodPost,
		URL:    "/orders",
		Body:   body,
	})
	require.Equal(t, http.StatusOK, res.Code)

	resp := trade.PlaceOrderResponse{}
	err := json.NewDecoder(res.Body).Decode(&resp)
	require.NoError(t, err)

	return resp
}

func getOrders(t testing.TB, client *testutil.Client) []models.Order {
	t.Helper()

	res := client.Request(&testutil.RequestOption{
		Method: http.MethodGet,
		URL:    "/orders",
	})
	require.Equal(t, http.StatusOK, res.Code)

	orders := make([]models.Order, 0)
	err := json.NewDecoder(res.Body).Decode(&orders)
	require.NoError(t, err)

	return orders
}

func requireValidOrderKey(t testing.TB, key string) {
	t.Helper()

	trimmed := strings.TrimSpace(key)
	require.NotEmpty(t, trimmed)
	_, err := base32.StdEncoding.DecodeString(trimmed)
	require.NoError(t, err)
}
