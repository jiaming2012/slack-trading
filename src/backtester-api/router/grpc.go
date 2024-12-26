package router

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	pb "github.com/jiaming2012/slack-trading/playground"
	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type Server struct{}

func convertOrders(orders []*models.BacktesterOrder) []*pb.Order {
	out := make([]*pb.Order, len(orders))

	for _, o := range orders {
		out = append(out, convertOrder(o))
	}

	return out
}

func convertOrder(o *models.BacktesterOrder) *pb.Order {
	var trades []*pb.Trade
	for _, trade := range o.Trades {
		trades = append(trades, &pb.Trade{
			Symbol:     trade.Symbol.GetTicker(),
			CreateDate: trade.CreateDate.String(),
			Quantity:   trade.Quantity,
			Price:      trade.Price,
		})
	}

	order := &pb.Order{
		Id:             uint64(o.ID),
		Class:          string(o.Class),
		Symbol:         o.Symbol.GetTicker(),
		Side:           string(o.Side),
		Quantity:       o.AbsoluteQuantity,
		Type:           string(o.Type),
		Duration:       string(o.Duration),
		RequestedPrice: o.RequestedPrice,
		Tag:            o.Tag,
		Trades:         trades,
		Status:         string(o.Status),
		CreateDate:     o.CreateDate.String(),
	}

	if o.Price != nil {
		order.Price = *o.Price
	}

	if o.StopPrice != nil {
		order.StopPrice = *o.StopPrice
	}

	if o.RejectReason != nil {
		order.RejectReason = *o.RejectReason
	}

	return order
}

func (s *Server) GetCandles(ctx context.Context, req *pb.GetCandlesRequest) (*pb.GetCandlesResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick: %v", err)
	}

	from, err := time.Parse(time.RFC3339, req.FromRTF3339)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick while parsing from timestamp: %v", err)
	}

	to, err := time.Parse(time.RFC3339, req.ToRTF3339)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick while parsing to timestamp: %v", err)
	}

	period := time.Duration(req.PeriodInSeconds) * time.Second

	candles, err := fetchCandles(playgroundId, eventmodels.StockSymbol(req.Symbol), period, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get candles: %v", err)
	}

	barsDTO := make([]*pb.Bar, 0)
	for _, candle := range candles {
		barsDTO = append(barsDTO, &pb.Bar{
			Open:     float32(candle.Open),
			High:     float32(candle.High),
			Low:      float32(candle.Low),
			Close:    float32(candle.Close),
			Volume:   float32(candle.Volume),
			Datetime: candle.Timestamp.String(),
		})
	}

	return &pb.GetCandlesResponse{
		Bars: barsDTO,
	}, nil
}

func (s *Server) NextTick(ctx context.Context, req *pb.NextTickRequest) (*pb.TickDelta, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick: %v", err)
	}

	duration := time.Duration(req.Seconds) * time.Second

	tick, err := nextTick(playgroundId, duration, req.IsPreview)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick: %v", err)
	}

	newTrades := make([]*pb.Trade, 0)
	for _, trade := range tick.NewTrades {
		newTrades = append(newTrades, &pb.Trade{
			Symbol:     trade.Symbol.GetTicker(),
			CreateDate: trade.CreateDate.String(),
			Quantity:   trade.Quantity,
			Price:      trade.Price,
		})
	}

	newCandles := make([]*pb.Candle, 0)
	for _, candle := range tick.NewCandles {
		dto := candle.Bar.ToDTO()
		newCandles = append(newCandles, &pb.Candle{
			Symbol: candle.Symbol.GetTicker(),
			Period: int32(candle.Period.Seconds()),
			Bar: &pb.Bar{
				Open:     float32(dto.Open),
				High:     float32(dto.High),
				Low:      float32(dto.Low),
				Close:    float32(dto.Close),
				Volume:   float32(dto.Volume),
				Datetime: dto.Timestamp,
			},
		})
	}

	invalidOrdersDTO := convertOrders(tick.InvalidOrders)

	tickDeltaEvents := make([]*pb.TickDeltaEvent, 0)
	for _, event := range tick.Events {
		var liquidationEvent *pb.LiquidationEvent

		if event.LiquidationEvent != nil {
			ordersPlaced := convertOrders(event.LiquidationEvent.OrdersPlaced)

			liquidationEvent = &pb.LiquidationEvent{
				OrdersPlaced: ordersPlaced,
			}

			tickDeltaEvents = append(tickDeltaEvents, &pb.TickDeltaEvent{
				Type:             string(models.TickDeltaEventTypeLiquidation),
				LiquidationEvent: liquidationEvent,
			})
		}
	}

	return &pb.TickDelta{
		NewTrades:          newTrades,
		NewCandles:         newCandles,
		InvalidOrders:      invalidOrdersDTO,
		Events:             tickDeltaEvents,
		CurrentTime:        tick.CurrentTime,
		IsBacktestComplete: tick.IsBacktestComplete,
	}, nil
}

func (s *Server) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %v", err)
	}

	account, err := getAccountInfo(playgroundId, req.FetchOrders)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %v", err)
	}

	positions := make(map[string]*pb.Position)
	for k, v := range account.Positions {
		positions[k] = &pb.Position{
			Quantity:          v.Quantity,
			CostBasis:         v.CostBasis,
			Pl:                v.PL,
			MaintenanceMargin: v.MaintenanceMargin,
		}
	}

	ordersDTO := convertOrders(account.Orders)

	return &pb.GetAccountResponse{
		Balance:    account.Balance,
		Equity:     account.Equity,
		FreeMargin: account.FreeMargin,
		Positions:  positions,
		Orders:     ordersDTO,
	}, nil
}

func (s *Server) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.Order, error) {
	playgroundID, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playground id: %v", err)
	}

	order, webErr := placeOrder(playgroundID, &CreateOrderRequest{
		Symbol:    req.Symbol,
		Class:     models.BacktesterOrderClass(req.AssetClass),
		Quantity:  req.Quantity,
		Side:      models.BacktesterOrderSide(req.Side),
		OrderType: models.BacktesterOrderType(req.Type),
		Price:     nil,
		StopPrice: nil,
		Duration:  models.BacktesterOrderDuration(req.Duration),
		Tag:       req.Tag,
	})

	if webErr != nil {
		return nil, fmt.Errorf("failed to place order: %v", webErr)
	}

	orderDTO := convertOrder(order)

	return orderDTO, nil
}

func (s *Server) CreatePlayground(ctx context.Context, req *pb.CreatePolygonPlaygroundRequest) (*pb.CreatePlaygroundResponse, error) {
	var repositoryRequests []CreateRepositoryRequest

	if len(req.Symbol) != len(req.TimespanMultiplier) || len(req.Symbol) != len(req.TimespanUnit) {
		return nil, fmt.Errorf("symbol, timespan multiplier, and timespan unit must have the same length")
	}

	for i := 0; i < len(req.Symbol); i++ {
		repositoryRequests = append(repositoryRequests, CreateRepositoryRequest{
			Symbol: req.Symbol[i],
			Timespan: PolygonTimespanRequest{
				Multiplier: int(req.TimespanMultiplier[i]),
				Unit:       req.TimespanUnit[i],
			},
			Source: RepositorySource{
				Type: RepositorySourcePolygon,
			},
		})
	}

	playground, err := createPlayground(&CreatePlaygroundRequest{
		Env:     req.GetEnvironment(),
		Balance: float64(req.Balance),
		Clock: CreateClockRequest{
			StartDate: req.StartDate,
			StopDate:  req.StopDate,
		},
		Repositories: repositoryRequests,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create playground: %v", err)
	}

	return &pb.CreatePlaygroundResponse{
		Id: playground.ID.String(),
	}, nil
}
