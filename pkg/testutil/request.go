package testutil

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strconv"

	"github.com/labstack/echo/v4"
)

type RequestOption struct {
	Method      string
	URL         string
	Body        any
	ContentType string
}

type Client struct {
	userId uint64
	server *echo.Echo
}

func NewClient(e *echo.Echo) *Client {
	return &Client{server: e}
}

func (c *Client) SetUser(userId uint64) {
	c.userId = userId
}

func (c *Client) Request(opts *RequestOption) *httptest.ResponseRecorder {
	var reqBody bytes.Buffer
	err := json.NewEncoder(&reqBody).Encode(opts.Body)
	if err != nil {
		panic(err)
	}
	req := httptest.NewRequest(opts.Method, opts.URL, &reqBody)
	if opts.ContentType != "" {
		req.Header.Set(echo.HeaderContentType, opts.ContentType)
	} else {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	req.Header.Set(echo.HeaderAuthorization, strconv.FormatUint(c.userId, 10))
	res := httptest.NewRecorder()
	c.server.ServeHTTP(res, req)

	return res
}
