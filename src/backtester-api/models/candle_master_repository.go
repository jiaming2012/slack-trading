package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type CandleMasterRepository struct {
	data          map[string]map[time.Duration]*CandleRepository
	instrumentMeta map[string]eventmodels.Instrument
}

func NewCandleMasterRepository(repos map[eventmodels.Instrument]map[time.Duration]*CandleRepository) *CandleMasterRepository {
	data := make(map[string]map[time.Duration]*CandleRepository)
	instrumentMeta := make(map[string]eventmodels.Instrument)

	for k, v := range repos {
		data[k.GetTicker()] = v
		instrumentMeta[k.GetTicker()] = k
	}
	
	return &CandleMasterRepository{
		data:          data,
		instrumentMeta: instrumentMeta,
	}
}

func (r *CandleMasterRepository) HasInstrument(instrument eventmodels.Instrument) bool {
	_, ok := r.data[instrument.GetTicker()]
	return ok
}

func (r *CandleMasterRepository) Add(instrument eventmodels.Instrument, period time.Duration, repo *CandleRepository) {
	if r.data == nil {
		r.data = make(map[string]map[time.Duration]*CandleRepository)
	}
	if _, ok := r.data[instrument.GetTicker()]; !ok {
		r.data[instrument.GetTicker()] = make(map[time.Duration]*CandleRepository)
	}
	r.data[instrument.GetTicker()][period] = repo
}

func (r *CandleMasterRepository) Get(instrument eventmodels.Instrument, period time.Duration) (*CandleRepository, bool) {
	if periodRepos, ok := r.data[instrument.GetTicker()]; ok {
		if repo, ok := periodRepos[period]; ok {
			return repo, true
		}
	}
	return nil, false
}

func (r *CandleMasterRepository) Iter() map[eventmodels.Instrument]map[time.Duration]*CandleRepository {
	out := make(map[eventmodels.Instrument]map[time.Duration]*CandleRepository)
	for k, v := range r.data {
		out[r.instrumentMeta[k]] = v
	}
	return out
}

func (r *CandleMasterRepository) Delete(instrument eventmodels.Instrument) {
	delete(r.data, instrument.GetTicker())
	delete(r.instrumentMeta, instrument.GetTicker())
}