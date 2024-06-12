package eventmodels

import "encoding/json"

type TradierOrdersPayloadDTO struct {
	Order json.RawMessage `json:"order"`
}
