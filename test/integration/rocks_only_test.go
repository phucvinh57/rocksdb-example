package engine_test

import (
	"encoding/base32"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"trading-bsx/cmd/api/server"
	"trading-bsx/internal/trade"
	"trading-bsx/pkg/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RocksOnly_PlaceAndGetOrders(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("MONGODB_ENABLED", "false")
	t.Setenv("ROCKSDB_IN_MEMORY", "true")

	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(42)

	res := client.Request(&testutil.RequestOption{
		Method: http.MethodPost,
		URL:    "/orders",
		Body: trade.CreateOrder{
			Type:  "BUY",
			Price: 123.45,
		},
	})
	require.Equal(t, http.StatusOK, res.Code)

	orderID := strings.TrimSpace(res.Body.String())
	_, err := base32.StdEncoding.DecodeString(orderID)
	require.NoError(t, err)

	res = client.Request(&testutil.RequestOption{
		Method: http.MethodGet,
		URL:    "/orders",
	})
	require.Equal(t, http.StatusOK, res.Code)

	var orders []map[string]any
	err = json.NewDecoder(res.Body).Decode(&orders)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, orderID, orders[0]["key"])
	assert.Equal(t, "BUY", orders[0]["type"])
}

func Test_RocksOnly_CancelOrder(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("MONGODB_ENABLED", "false")
	t.Setenv("ROCKSDB_IN_MEMORY", "true")

	s := server.New()
	defer s.Close()

	client := testutil.NewClient(s)
	client.SetUser(7)

	res := client.Request(&testutil.RequestOption{
		Method: http.MethodPost,
		URL:    "/orders",
		Body: trade.CreateOrder{
			Type:  "SELL",
			Price: 99.9,
		},
	})
	require.Equal(t, http.StatusOK, res.Code)

	orderID := strings.TrimSpace(res.Body.String())
	res = client.Request(&testutil.RequestOption{
		Method: http.MethodDelete,
		URL:    fmt.Sprintf("/orders/%s", orderID),
	})
	require.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, orderID, strings.TrimSpace(res.Body.String()))

	res = client.Request(&testutil.RequestOption{
		Method: http.MethodGet,
		URL:    "/orders",
	})
	require.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "[]", strings.TrimSpace(res.Body.String()))
}
