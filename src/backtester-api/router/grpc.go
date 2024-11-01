package router

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	pb "github.com/jiaming2012/slack-trading/src/backtester-api/playground"
)

type GrpcServer struct {
	pb.UnimplementedPlaygroundServiceServer
}

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
		Quantity:       float32(o.AbsoluteQuantity),
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

func (s *GrpcServer) NextTick(ctx context.Context, req *pb.NextTickRequest) (*pb.TickDelta, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick: %v", err)
	}

	tick, err := nextTick(playgroundId, time.Duration(req.Seconds), req.IsPreview)
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
		newCandles = append(newCandles, &pb.Candle{
			Symbol: candle.Symbol.GetTicker(),
			Bar: &pb.Bar{
				Open:  float32(candle.Bar.Open),
				High:  float32(candle.Bar.High),
				Low:   float32(candle.Bar.Low),
				Close: float32(candle.Bar.Close),
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

func (s *GrpcServer) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
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
			Quantity:          float32(v.Quantity),
			CostBasis:         v.CostBasis,
			Pl:                v.PL,
			MaintenanceMargin: float32(v.MaintenanceMargin),
		}
	}

	ordersDTO := convertOrders(account.Orders)

	return &pb.GetAccountResponse{
		Balance:    float32(account.Balance),
		Equity:     float32(account.Equity),
		FreeMargin: float32(account.FreeMargin),
		Positions:  positions,
		Orders:     ordersDTO,
	}, nil
}

func (s *GrpcServer) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.Order, error) {
	playgroundID, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playground id: %v", err)
	}

	tag := ""

	order, webErr := placeOrder(playgroundID, &CreateOrderRequest{
		Symbol:    req.Symbol,
		Class:     models.BacktesterOrderClass(req.AssetClass),
		Quantity:  req.Quantity,
		Side:      models.BacktesterOrderSide(req.Side),
		OrderType: models.BacktesterOrderType(req.Type),
		Price:     nil,
		StopPrice: nil,
		Duration:  models.BacktesterOrderDuration(req.Duration),
		Tag:       tag,
	})

	if webErr != nil {
		return nil, fmt.Errorf("failed to place order: %v", webErr)
	}

	orderDTO := convertOrder(order)

	return orderDTO, nil
}

func (s *GrpcServer) CreatePlayground(ctx context.Context, req *pb.CreatePolygonPlaygroundRequest) (*pb.CreatePlaygroundResponse, error) {
	playground, err := createPlayground(&CreatePlaygroundRequest{
		Balance: float64(req.Balance),
		Clock: CreateClockRequest{
			StartDate: req.StartDate,
			StopDate:  req.StopDate,
		},
		Repository: CreateRepositoryRequest{
			Symbol: req.Symbol,
			Timespan: PolygonTimespanRequest{
				Multiplier: int(req.TimespanMultiplier),
				Unit:       req.TimespanUnit,
			},
			Source: RepositorySource{
				Type: RepositorySourcePolygon,
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create playground: %v", err)
	}

	return &pb.CreatePlaygroundResponse{
		Id: playground.ID.String(),
	}, nil
}
