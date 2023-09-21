package http_client

import (
	"github.com/go-resty/resty/v2"
	"strconv"
	"sync"
	"time"
)

var clientOnce sync.Once
var httpClient *HttpClient

type HttpClient struct {
	Client *resty.Client
}

func NewHttpClient() *HttpClient {
	clientOnce.Do(func() {
		client := resty.New()
		httpClient.Client = client
	})
	return httpClient
}

func (h *HttpClient) Get(url string) (*resty.Response, error) {
	response, err := h.Client.R().Get(url)
	if err != nil {
		return nil, err
	}
	return response, err
}

func (h *HttpClient) GetQueryParams(url string, params map[string]string) (*resty.Response, error) {
	response, err := h.Client.R().SetQueryParams(map[string]string{
		"page_no": "1",
		"limit":   "20",
		"sort":    "name",
		"order":   "asc",
		"random":  strconv.FormatInt(time.Now().Unix(), 10),
	}).SetHeader("Content-Type", "application/json").Get(url)
	if err != nil {
		return nil, err
	}
	return response, err
}

func (h *HttpClient) Post(url string, body interface{}) (*resty.Response, error) {
	response, err := h.Client.R().SetHeader("Content-Type", "application/json").SetBody(body).Post(url)
	if err != nil {
		return nil, err
	}
	return response, err
}
