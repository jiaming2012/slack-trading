package router

// proto_converters.go centralizes all model -> proto DTO conversions used by
// the Twirp handlers in grpc.go. These functions are pure extractions: field
// mappings, nil-handling, and ordering are byte-for-byte identical to the
// original inline conversion logic.

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jiaming2012/slack-trading/src/go/backtester-api/models"
	eventmodels "github.com/jiaming2012/slack-trading/src/go/models"
	pb "github.com/jiaming2012/slack-trading/src/go/playground"
)

// ordersToProto converts a slice of order records to proto Orders, skipping
// any nil conversions. (Formerly convertOrders in grpc.go.)
func ordersToProto(orders []*models.OrderRecord, externalIdMap map[uint]*models.OrderRecord) []*pb.Order {
	out := make([]*pb.Order, 0)

	for _, order := range orders {
		if o := orderToProto(order, externalIdMap); o != nil {
			out = append(out, o)
		}
	}

	return out
}

// orderToProto converts an order record (including its trades, closes,
// reconciles, and previous position) to a proto Order. When externalIdMap is
// non-nil, the external id is resolved from the map; otherwise it falls back
// to the order's own ExternalOrderID. (Formerly convertOrder in grpc.go.)
func orderToProto(o *models.OrderRecord, externalIdMap map[uint]*models.OrderRecord) *pb.Order {
	var trades []*pb.Trade
	for _, trade := range o.GetTrades() {
		trades = append(trades, tradeToProto(trade))
	}

	var closes []*pb.Order
	for _, order := range o.Closes {
		closes = append(closes, orderToProto(order, externalIdMap))
	}

	var pl *float64
	if len(closes) > 0 {
		calculatedPL := o.CalcRealizedPL()
		pl = &calculatedPL
	}

	var closedBy []*pb.Trade
	for _, trade := range o.ClosedBy {
		closedBy = append(closedBy, &pb.Trade{
			Id:         uint64(trade.ID),
			CreateDate: trade.Timestamp.String(),
			Quantity:   trade.Quantity,
			Price:      trade.Price,
		})
	}

	var reconciles []*pb.Order
	for _, order := range o.Reconciles {
		reconciles = append(reconciles, orderToProto(order, externalIdMap))
	}

	var externalId *uint64
	if externalIdMap != nil {
		if reconcileOrder, ok := externalIdMap[o.ID]; ok {
			_id := uint64(*reconcileOrder.ExternalOrderID)
			externalId = &_id
		}
	} else if o.ExternalOrderID != nil {
		_externalId := uint64(*o.ExternalOrderID)
		externalId = &_externalId
	}

	previousPosition := positionToProto(&o.PreviousPosition)

	var closeOrderId *uint64
	if o.CloseOrderId != nil {
		_closeOrderId := uint64(*o.CloseOrderId)
		closeOrderId = &_closeOrderId
	}

	var signalIdStr *string
	if o.SignalID != nil {
		s := o.SignalID.String()
		signalIdStr = &s
	}

	order := &pb.Order{
		Id:               uint64(o.ID),
		ExternalId:       externalId,
		ClientRequestId:  o.ClientRequestID,
		Class:            string(o.Class),
		Symbol:           o.GetInstrument().GetTicker(),
		Side:             string(o.Side),
		Quantity:         o.AbsoluteQuantity,
		Type:             string(o.OrderType),
		Duration:         string(o.Duration),
		RequestedPrice:   o.RequestedPrice,
		Tag:              o.Tag,
		Trades:           trades,
		Status:           string(o.Status),
		CreateDate:       o.Timestamp.String(),
		ClosedBy:         closedBy,
		Closes:           closes,
		Reconciles:       reconciles,
		PreviousPosition: previousPosition,
		CloseOrderId:     closeOrderId,
		Attributes:       o.Attributes,
		PreviousBalance:  o.PreviousBalance,
		Pl:               pl,
		SignalId:         signalIdStr,
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

// tradeToProto converts a trade record to a proto Trade, including its
// optional order and reconcile-order references.
func tradeToProto(trade *models.TradeRecord) *pb.Trade {
	var orderId *uint64
	if trade.OrderID != nil {
		_orderId := uint64(*trade.OrderID)
		orderId = &_orderId
	}

	var reconcileOrderId *uint64
	if trade.ReconcileOrderID != nil {
		_reconcileOrderId := uint64(*trade.ReconcileOrderID)
		reconcileOrderId = &_reconcileOrderId
	}

	return &pb.Trade{
		Id:               uint64(trade.ID),
		CreateDate:       trade.Timestamp.String(),
		Quantity:         trade.Quantity,
		Price:            trade.Price,
		OrderId:          orderId,
		ReconcileOrderId: reconcileOrderId,
	}
}

// positionToProto converts a single position to a proto Position.
func positionToProto(p *models.Position) *pb.Position {
	return &pb.Position{
		Quantity:          p.Quantity,
		CostBasis:         p.CostBasis,
		Pl:                p.PL,
		MaintenanceMargin: p.MaintenanceMargin,
		CurrentPrice:      p.CurrentPrice,
		Timestamp:         p.Timestamp,
	}
}

// positionsToProto converts a symbol-keyed position map to its proto
// equivalent. Always returns a non-nil map.
func positionsToProto(positions map[string]*models.Position) map[string]*pb.Position {
	out := make(map[string]*pb.Position)
	for symbol, pos := range positions {
		out[symbol] = positionToProto(pos)
	}
	return out
}

// candleToProto converts a backtester candle to a proto Candle.
func candleToProto(c *models.BacktesterCandle) *pb.Candle {
	return &pb.Candle{
		Symbol: c.Symbol.GetTicker(),
		Period: int32(c.Period.Seconds()),
		Bar:    c.Bar.ToProto(),
	}
}

// tradeSignalToProto converts a trade signal to a proto TradeSignalProto,
// stringifying all attribute values.
func tradeSignalToProto(sig *eventmodels.TradeSignal) *pb.TradeSignalProto {
	attrs := make(map[string]string, len(sig.Attributes))
	for k, v := range sig.Attributes {
		attrs[k] = fmt.Sprintf("%v", v)
	}
	return &pb.TradeSignalProto{
		Id:         sig.ID.String(),
		Name:       string(sig.Name),
		Symbol:     string(sig.Symbol),
		Timestamp:  timestamppb.New(sig.Timestamp),
		Attributes: attrs,
	}
}

// liquidationEventToProto wraps a liquidation event in a proto TickDeltaEvent.
func liquidationEventToProto(ev *models.LiquidationEvent) *pb.TickDeltaEvent {
	return &pb.TickDeltaEvent{
		Type: string(models.TickDeltaEventTypeLiquidation),
		LiquidationEvent: &pb.LiquidationEvent{
			OrdersPlaced: ordersToProto(ev.OrdersPlaced, nil),
		},
	}
}

// optionExpirationEventToProto wraps an option expiration event in a proto
// TickDeltaEvent, decomposing the option symbol into its components.
func optionExpirationEventToProto(ev *models.OptionExpirationEvent) (*pb.TickDeltaEvent, error) {
	components, err := ev.Symbol.Components()
	if err != nil {
		return nil, fmt.Errorf("failed to get option components: %v", err)
	}

	return &pb.TickDeltaEvent{
		Type: string(models.TickDeltaEventTypeOptionExpired),
		OptionExpirationEvent: &pb.OptionExpirationEvent{
			OptionSymbol:                string(components.Symbol.GetTicker()),
			UnderlyingSymbol:            string(components.Underlying),
			Timestamp:                   ev.Timestamp.Format(time.RFC3339),
			ExpirationDate:              components.Expiration.Format(time.RFC3339),
			UnderlyingPriceAtExpiration: ev.UnderlyingPriceAtExpiry,
			Strike:                      components.StrikePrice,
		},
	}, nil
}

// optionAssignmentEventToProto wraps an option assignment event in a proto
// TickDeltaEvent.
func optionAssignmentEventToProto(ev *models.OptionAssignmentEvent) *pb.TickDeltaEvent {
	return &pb.TickDeltaEvent{
		Type: string(models.TickDeltaEventTypeOptionAssigned),
		OptionAssignmentEvent: &pb.OptionAssignmentEvent{
			OrderId:          uint64(ev.OrderId),
			Symbol:           ev.Symbol.GetTicker(),
			Timestamp:        ev.Timestamp.Format(time.RFC3339),
			AssignedQuantity: ev.AssignedQuantity,
			AssignedPrice:    ev.AssignedPrice,
		},
	}
}

// tickDeltaToProto converts a tick delta to its proto equivalent.
// NOTE: intentionally only surfaces liquidation and option-expiration events,
// matching NextTick's original behavior (option-assignment events are only
// surfaced by GetAccount via accountEventsToProto).
func tickDeltaToProto(tick *models.TickDelta) (*pb.TickDelta, error) {
	newTrades := make([]*pb.Trade, 0)
	for _, trade := range tick.NewTrades {
		newTrades = append(newTrades, tradeToProto(trade))
	}

	newCandles := make([]*pb.Candle, 0)
	for _, c := range tick.NewCandles {
		newCandles = append(newCandles, candleToProto(c))
	}

	invalidOrdersDTO := ordersToProto(tick.InvalidOrders, nil)

	tickDeltaEvents := make([]*pb.TickDeltaEvent, 0)
	for _, event := range tick.Events {
		if event.LiquidationEvent != nil {
			tickDeltaEvents = append(tickDeltaEvents, liquidationEventToProto(event.LiquidationEvent))
		}

		if event.OptionExpirationEvent != nil {
			expiredEvent, err := optionExpirationEventToProto(event.OptionExpirationEvent)
			if err != nil {
				return nil, err
			}

			tickDeltaEvents = append(tickDeltaEvents, expiredEvent)
		}
	}

	positions := positionsToProto(tick.Positions)

	newSignals := make([]*pb.TradeSignalProto, 0, len(tick.NewSignals))
	for _, sig := range tick.NewSignals {
		newSignals = append(newSignals, tradeSignalToProto(sig))
	}

	return &pb.TickDelta{
		NewTrades:          newTrades,
		NewCandles:         newCandles,
		InvalidOrders:      invalidOrdersDTO,
		Events:             tickDeltaEvents,
		CurrentTime:        tick.CurrentTime,
		IsBacktestComplete: tick.IsBacktestComplete,
		Balance:            tick.Balance,
		Equity:             tick.Equity,
		FreeMargin:         tick.FreeMargin,
		Positions:          positions,
		NewSignals:         newSignals,
	}, nil
}

// accountEventsToProto converts account tick-delta events (liquidation,
// option expiration, and option assignment) to proto TickDeltaEvents.
// Returns a nil slice when there are no events, matching GetAccount's
// original behavior.
func accountEventsToProto(accountEvents []*models.TickDeltaEvent) ([]*pb.TickDeltaEvent, error) {
	var events []*pb.TickDeltaEvent
	for _, event := range accountEvents {
		if event.LiquidationEvent != nil {
			events = append(events, liquidationEventToProto(event.LiquidationEvent))
		}

		if event.OptionExpirationEvent != nil {
			expiredEvent, err := optionExpirationEventToProto(event.OptionExpirationEvent)
			if err != nil {
				return nil, err
			}

			events = append(events, expiredEvent)
		}

		if event.OptionAssignmentEvent != nil {
			events = append(events, optionAssignmentEventToProto(event.OptionAssignmentEvent))
		}
	}
	return events, nil
}

// accountToProto converts an account response to its proto equivalent. Orders
// are passed in pre-converted because the caller controls the external-id
// resolution (see GetAccount's FetchExternalId handling).
func accountToProto(account *models.GetAccountResponse, orders []*pb.Order) (*pb.GetAccountResponse, error) {
	positions := positionsToProto(account.Positions)

	var endAt *string
	if account.Meta.EndAt != nil {
		_endAt := account.Meta.EndAt.Format(time.RFC3339)
		endAt = &_endAt
	}

	var liveAccountType *string
	if err := account.Meta.LiveAccountType.Validate(); err == nil {
		liveAccountType = new(string)
		*liveAccountType = string(account.Meta.LiveAccountType)
	}

	events, err := accountEventsToProto(account.Events)
	if err != nil {
		return nil, err
	}

	return &pb.GetAccountResponse{
		Meta: &pb.AccountMeta{
			PlaygroundId:          account.Meta.PlaygroundId,
			ReconcilePlaygroundId: account.Meta.ReconcilePlaygroundId,
			InitialBalance:        account.Meta.InitialBalance,
			StartDate:             account.Meta.StartAt.Format(time.RFC3339),
			EndDate:               endAt,
			Symbols:               account.Meta.Symbols,
			Environment:           string(account.Meta.Environment),
			LiveAccountType:       liveAccountType,
			Tags:                  account.Meta.Tags,
			ClientId:              account.Meta.ClientID,
			CurrentTime:           account.Meta.CurrentTime.Format(time.RFC3339),
		},
		Balance:    account.Balance,
		Equity:     account.Equity,
		FreeMargin: account.FreeMargin,
		Positions:  positions,
		Orders:     orders,
		Events:     events,
	}, nil
}

// repositoryToProto converts a candle repository to a proto Repository.
func repositoryToProto(repo *models.CandleRepository) *pb.Repository {
	return &pb.Repository{
		Symbol:             repo.GetSymbol().GetTicker(),
		TimespanMultiplier: uint32(repo.GetPolygonTimespan().Multiplier),
		TimespanUnit:       string(repo.GetPolygonTimespan().Unit),
		Indicators:         repo.GetIndicators(),
		HistoryInDays:      repo.GetHistoryInDays(),
	}
}

// playgroundToProto converts a playground to a proto PlaygroundSession,
// refreshing prices/positions in the process (as GetPlaygrounds always did).
func playgroundToProto(p *models.Playground) (*pb.PlaygroundSession, error) {
	meta := p.GetMeta()
	positionCache, err := p.UpdatePricesAndGetPositionCache()
	if err != nil {
		return nil, fmt.Errorf("failed to get playground positions: %v", err)
	}

	balance := p.GetBalance()
	equity := p.GetEquity(positionCache)
	freeMargin, err := p.GetFreeMargin()
	if err != nil {
		return nil, fmt.Errorf("failed to get playground free margin: %v", err)
	}

	positionsDTO := positionsToProto(positionCache.Iter())

	var clockStop *string
	if meta.EndAt != nil {
		_stop := meta.EndAt.Format(time.RFC3339)
		clockStop = &_stop
	}

	var repos []*pb.Repository
	for _, repo := range p.GetRepositories() {
		repos = append(repos, repositoryToProto(repo))
	}

	var liveAccountType *string
	if err := meta.LiveAccountType.Validate(); err == nil {
		liveAccountType = new(string)
		*liveAccountType = string(meta.LiveAccountType)
	}

	var reconcilePlaygroundId *string
	if p.ReconcilePlayground != nil {
		_reconcilePlaygroundId := p.ReconcilePlayground.GetId().String()
		reconcilePlaygroundId = &_reconcilePlaygroundId
	}

	createdOn := p.CreatedAt.Format(time.RFC3339)

	return &pb.PlaygroundSession{
		PlaygroundId: p.GetId().String(),
		Meta: &pb.AccountMeta{
			PlaygroundId:          p.GetId().String(),
			ReconcilePlaygroundId: reconcilePlaygroundId,
			InitialBalance:        meta.InitialBalance,
			Environment:           string(meta.Environment),
			LiveAccountType:       liveAccountType,
			Tags:                  meta.Tags,
			ClientId:              p.ClientID,
			CreatedAt:             createdOn,
		},
		Clock: &pb.Clock{
			Start:       meta.StartAt.Format(time.RFC3339),
			Stop:        clockStop,
			CurrentTime: p.GetCurrentTime().Format(time.RFC3339),
		},
		Repositories: repos,
		Balance:      balance,
		Equity:       equity,
		FreeMargin:   freeMargin,
		Positions:    positionsDTO,
	}, nil
}

// optionLadderContractsToProto converts option chain contracts to proto
// OptionLadderContracts.
func optionLadderContractsToProto(optionContracts []eventmodels.OptionContractV3) []*pb.OptionLadderContract {
	var contracts []*pb.OptionLadderContract
	for _, contract := range optionContracts {
		contracts = append(contracts, &pb.OptionLadderContract{
			Symbol:         string(contract.Symbol),
			ExpirationDate: string(contract.ExpirationDate),
			Timestamp:      contract.Timestamp.Format(time.RFC3339),
			Strike:         contract.Strike,
			Type:           string(contract.OptionType),
			Bid:            contract.Bid,
			Ask:            contract.Ask,
			ContractSize:   float64(contract.ContractSize),
		})
	}
	return contracts
}

// liveAccountPlotsToProto converts equity report items to proto
// LiveAccountPlots. Returns an error when an item has a nil equity.
func liveAccountPlotsToProto(plots []models.LiveAccountPlot) ([]*pb.LiveAccountPlot, error) {
	var items []*pb.LiveAccountPlot
	for _, item := range plots {
		if item.Equity == nil {
			return nil, fmt.Errorf("equity is nil")
		}

		items = append(items, &pb.LiveAccountPlot{
			Timestamp: item.Timestamp.Format(time.RFC3339),
			Equity:    *item.Equity,
		})
	}
	return items, nil
}

// equityPlotsToProto converts equity plot points to proto EquityPlots.
// Always returns a non-nil slice.
func equityPlotsToProto(plots []*eventmodels.EquityPlot) []*pb.EquityPlot {
	out := make([]*pb.EquityPlot, 0)
	for _, p := range plots {
		out = append(out, &pb.EquityPlot{
			CreatedAt: p.Timestamp.Format(time.RFC3339),
			Equity:    p.Value,
		})
	}
	return out
}

// positionReportsToProto converts a position cache to proto PositionReports,
// stamping each report with the given playground id pointer.
func positionReportsToProto(cache *models.PositionsCache, playgroundId *string) []*pb.PositionReport {
	var reports []*pb.PositionReport
	for symbol, p := range cache.Iter() {
		reports = append(reports, &pb.PositionReport{
			Symbol:       symbol,
			Quantity:     p.Quantity,
			PlaygroundId: playgroundId,
		})
	}
	return reports
}

// tradierPositionReportsToProto converts broker position DTOs to proto
// PositionReports (no playground id — these are broker-side positions).
func tradierPositionReportsToProto(positions []eventmodels.TradierPositionDTO) []*pb.PositionReport {
	var reports []*pb.PositionReport
	for _, p := range positions {
		reports = append(reports, &pb.PositionReport{
			Symbol:   p.Symbol,
			Quantity: p.Quantity,
		})
	}
	return reports
}

// repositoryRequestsFromProto converts proto Repository specs into repository
// creation requests with the given source type. (proto -> model direction;
// shared by CreatePlayground and CreateLivePlayground.)
func repositoryRequestsFromProto(repos []*pb.Repository, sourceType eventmodels.RepositorySourceType) []eventmodels.CreateRepositoryRequest {
	var requests []eventmodels.CreateRepositoryRequest
	for _, repo := range repos {
		requests = append(requests, eventmodels.CreateRepositoryRequest{
			Symbol: repo.Symbol,
			Timespan: eventmodels.PolygonTimespanRequest{
				Multiplier: int(repo.TimespanMultiplier),
				Unit:       repo.TimespanUnit,
			},
			Source: eventmodels.RepositorySource{
				Type: sourceType,
			},
			Indicators:    repo.Indicators,
			HistoryInDays: repo.HistoryInDays,
		})
	}
	return requests
}
