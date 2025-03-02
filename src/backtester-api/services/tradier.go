package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type TradierBroker struct {
	ordersUrl      string
	quotesUrl      string
	nonTradesToken string
	tradesToken    string
}

func (b *TradierBroker) FetchQuotes(ctx context.Context, symbols []eventmodels.Instrument) ([]*models.TradierQuoteDTO, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("no symbols provided")
	}

	dto, err := FetchQuotes(ctx, b.quotesUrl, b.nonTradesToken, symbols)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quotes: %w", err)
	}

	return dto, nil
}

func (b *TradierBroker) FetchOrders(ctx context.Context) ([]*eventmodels.TradierOrder, error) {
	dto, err := FetchOrders(ctx, b.ordersUrl, b.tradesToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}

	orders := make([]*eventmodels.TradierOrder, 0, len(dto))
	for _, orderDTO := range dto {
		order, err := orderDTO.ToTradierOrder()
		if err != nil {
			return nil, fmt.Errorf("failed to convert order dto to order: %w", err)
		}

		orders = append(orders, order)
	}

	return orders, nil
}

func (b *TradierBroker) PlaceOrder(ctx context.Context, req *models.PlaceEquityTradeRequest) (map[string]interface{}, error) {

	resp, err := PlaceOrder(ctx, b.ordersUrl, b.tradesToken, req)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	return resp, nil
}

func NewTradierBroker(ordersUrl, quotesUrl, nonTradesToken, tradesToken string) *TradierBroker {
	return &TradierBroker{
		ordersUrl:      ordersUrl,
		quotesUrl:      quotesUrl,
		nonTradesToken: nonTradesToken,
		tradesToken:    tradesToken,
	}
}

func FetchQuotes(ctx context.Context, baseUrl, token string, symbols []eventmodels.Instrument) ([]*models.TradierQuoteDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	symbolsStr := make([]string, 0, len(symbols))
	for _, s := range symbols {
		symbolsStr = append(symbolsStr, s.GetTicker())
	}

	queryParams := url.Values{}
	symbolsCommadSeparated := strings.Join(symbolsStr, ",")
	queryParams.Add("symbols", symbolsCommadSeparated)

	fullUrl := fmt.Sprintf("%s?%s", baseUrl, queryParams.Encode())

	req, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchQuotes: failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	log.Tracef("fetching quotes from %s", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FetchQuotes: query failed: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		errBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("FetchQuotes: failed to read response body: %w", err)
		}

		log.Errorf("FetchQuotes: failed to fetch quotes: %s", string(errBytes))

		return nil, fmt.Errorf("FetchQuotes: invalid status code: %s", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("FetchQuotes: failed to read response body: %w", err)
	}

	return utils.ParseTradierResponse[*models.TradierQuoteDTO](bytes)
}

func FetchOrders(ctx context.Context, baseUrl, token string) ([]*eventmodels.TradierOrderDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	queryParams := url.Values{}
	queryParams.Add("includeTags", "true")

	fullUrl := fmt.Sprintf("%s?%s", baseUrl, queryParams.Encode())

	req, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchOrders:fetchOrders(): failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	log.Debugf("fetching orders from %s", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FetchOrders:fetchOrders(): failed to fetch option prices: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchOrders:fetchOrders(): failed to fetch option prices: %s", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("FetchOrders:fetchOrders(): failed to read response body: %w", err)
	}

	orders, err := utils.ParseTradierResponse[*eventmodels.TradierOrderDTO](bytes)
	if err != nil {
		return nil, fmt.Errorf("FetchOrders:fetchOrders(): failed to parse response body: %w", err)
	}

	return orders, nil
}

func PlaceOrder(ctx context.Context, url, token string, req *models.PlaceEquityTradeRequest) (map[string]interface{}, error) {
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("PlaceOrder: quantity must be positive")
	}

	quantityStr := strconv.Itoa(req.Quantity)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("PlaceOrder: failed to create request: %w", err)
	}

	symbol := strings.ToUpper(req.Symbol)

	q := httpReq.URL.Query()
	q.Add("class", "equity")
	q.Add("type", string(req.OrderType))
	q.Add("duration", string(eventmodels.TradeDurationDay))
	q.Add("symbol", symbol)
	q.Add("quantity", quantityStr)
	q.Add("side", string(req.Side))

	if req.Tag != "" {
		q.Add("tag", req.Tag)
	}

	if req.DryRun {
		q.Add("preview", "true")
	}

	httpReq.URL.RawQuery = q.Encode()
	httpReq.Header.Add("Accept", "application/json")
	httpReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	log.Infof("PlaceOrder: placing trade: %v", httpReq.URL.String())

	res, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("PlaceOrder: failed to place trade: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bytesErr, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("PlaceOrder: failed to read response body: %w", err)
		}

		return nil, fmt.Errorf("PlaceOrder: %s, http code %v", string(bytesErr), res.Status)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("PlaceOrder: failed to decode response: %w", err)
	}

	if e, found := response["errors"]; found {
		return nil, fmt.Errorf("PlaceOrder: failed to place trade: %v", e)
	}

	log.Infof("PlaceOrder: placed trade: %v", response)

	return response, nil
}
