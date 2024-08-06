package eventservices

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func Serve_ReadOptionChainRequestExecutor(s *eventmodels.ReadOptionChainRequestExecutor, r *http.Request, request eventmodels.ApiRequest3, resultCh chan map[string]interface{}, errorCh chan error) {
	req := request.(*eventmodels.ReadOptionChainRequest)

	bFindSpreads := false
	if r.URL.Path == "/options/spreads" {
		bFindSpreads = true
	}

	if req.EV != nil {
		// data, err := s.CollectData(r.Context(), req)
		now := time.Now()
		expirationGTE := now
		nextOptionsExpirationDate := utils.DeriveNextFriday(expirationGTE)

		data, err := s.OptionsDataFetcher.FetchOptionChainDataInput(req.Symbol, req.IsHistorical, now, expirationGTE, nextOptionsExpirationDate, 0, 0, []int{})

		if err != nil {
			errorCh <- fmt.Errorf("tradier executer: %v: failed to collect data: %v", req.Symbol, err)
			return
		}
		go s.ServeWithParams(r.Context(), req, *data, bFindSpreads, now, resultCh, errorCh)
	} else {
		// go s.serve(req, resultCh, errorCh)
		panic("not implemented")
	}
}
