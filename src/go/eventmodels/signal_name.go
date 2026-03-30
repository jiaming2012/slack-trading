package eventmodels

import "fmt"

type SignalName string

const (
	SuperTrendBuy                 SignalName = "supertrend-buy"
	SuperTrendSell                SignalName = "supertrend-sell"
	StochasticRsiSell             SignalName = "stochastic_rsi-sell"
	SuperTrend4h1hStochRsi15mUp   SignalName = "supertrend-4h-1h_stoch_rsi_15m_up"
	SuperTrend1hStochRsi15mUp     SignalName = "supertrend-1h_stoch_rsi_15m_up"
	SuperTrend4h1hStochRsi15mDown SignalName = "supertrend-4h-1h_stoch_rsi_15m_down"
	SuperTrend1hStochRsi15mDown   SignalName = "supertrend-1h_stoch_rsi_15m_down"

	SignalMACrossover   SignalName = "ma_crossover"
	SignalStartOfWeek   SignalName = "start_of_week"
	SignalCoveredCall   SignalName = "covered_call"
	SignalMeanReversion SignalName = "mean_reversion"
)

func (s SignalName) Validate() error {
	switch s {
	case SignalMACrossover, SignalStartOfWeek, SignalCoveredCall, SignalMeanReversion:
		return nil
	default:
		return fmt.Errorf("unknown signal name: %s", s)
	}
}
