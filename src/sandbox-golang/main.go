package main

import (
	"encoding/json"
	"fmt"

	"slack-trading/src/eventmodels"
)

func main() {
	// input := []byte(`
	// 	{
	// 		"quotes": {
	// 			"quote": [
	// 				{
	// 					"symbol": "COIN240328P00060000",
	// 					"description": "COIN Mar 28 2024 $60.00 Put",
	// 					"exch": "Z",
	// 					"type": "option",
	// 					"last": 0.01,
	// 					"change": 0.00,
	// 					"volume": 1,
	// 					"open": 0.01,
	// 					"high": 0.01,
	// 					"low": 0.01,
	// 					"close": 0.01,
	// 					"bid": 0.0,
	// 					"ask": 0.01,
	// 					"underlying": "COIN",
	// 					"strike": 60.0,
	// 					"greeks": {
	// 						"delta": -1.9785489425E-6,
	// 						"gamma": 2.0795020742955597E-6,
	// 						"theta": 2.401870318626865E-13,
	// 						"vega": 2.0000001027016188E-5,
	// 						"rho": 0.0,
	// 						"phi": 0.0,
	// 						"bid_iv": 0.0,
	// 						"mid_iv": 0.0,
	// 						"ask_iv": 0.0,
	// 						"smv_vol": 45.378,
	// 						"updated_at": "2024-03-22 19:59:53"
	// 					},
	// 					"change_percentage": 0.00,
	// 					"average_volume": 0,
	// 					"last_volume": 1,
	// 					"trade_date": 1711114212085,
	// 					"prevclose": 0.01,
	// 					"week_52_high": 0.0,
	// 					"week_52_low": 0.0,
	// 					"bidsize": 0,
	// 					"bidexch": null,
	// 					"bid_date": 1711108801000,
	// 					"asksize": 39,
	// 					"askexch": "X",
	// 					"ask_date": 1711114212000,
	// 					"open_interest": 154,
	// 					"contract_size": 100,
	// 					"expiration_date": "2024-03-28",
	// 					"expiration_type": "quarterlys",
	// 					"option_type": "put",
	// 					"root_symbol": "COIN"
	// 				},
	// 				{
	// 					"symbol": "COIN240328C00060000",
	// 					"description": "COIN Mar 28 2024 $60.00 Call",
	// 					"exch": "Z",
	// 					"type": "option",
	// 					"last": 192.61,
	// 					"change": 0.00,
	// 					"volume": 0,
	// 					"open": null,
	// 					"high": null,
	// 					"low": null,
	// 					"close": null,
	// 					"bid": 193.9,
	// 					"ask": 197.6,
	// 					"underlying": "COIN",
	// 					"strike": 60.0,
	// 					"greeks": {
	// 						"delta": 0.9999980214510575,
	// 						"gamma": 2.0795020742955597E-6,
	// 						"theta": 2.401870318626865E-13,
	// 						"vega": 2.0000001027016188E-5,
	// 						"rho": 0.0,
	// 						"phi": 0.0,
	// 						"bid_iv": 0.0,
	// 						"mid_iv": 6.271818,
	// 						"ask_iv": 6.271818,
	// 						"smv_vol": 45.378,
	// 						"updated_at": "2024-03-22 19:59:53"
	// 					},
	// 					"change_percentage": 0.00,
	// 					"average_volume": 0,
	// 					"last_volume": 1,
	// 					"trade_date": 1710961612943,
	// 					"prevclose": 192.61,
	// 					"week_52_high": 0.0,
	// 					"week_52_low": 0.0,
	// 					"bidsize": 50,
	// 					"bidexch": "Z",
	// 					"bid_date": 1711137552000,
	// 					"asksize": 47,
	// 					"askexch": "Z",
	// 					"ask_date": 1711137551000,
	// 					"open_interest": 6,
	// 					"contract_size": 100,
	// 					"expiration_date": "2024-03-28",
	// 					"expiration_type": "quarterlys",
	// 					"option_type": "call",
	// 					"root_symbol": "COIN"
	// 				}
	// 			],
	// 			"unmatched_symbols": {
	// 				"symbol": [
	// 					"COIN240322C00285000",
	// 					"COIN240315C00262500",
	// 					"COIN240315C00262500"
	// 				]
	// 			}
	// 		}
	// 	}
	// `)

	// input := []byte(`
	// 	{
	// 		"quotes": {
	// 			"quote": {
	// 				"symbol": "COIN240328P00060000",
	// 				"description": "COIN Mar 28 2024 $60.00 Put",
	// 				"exch": "Z",
	// 				"type": "option",
	// 				"last": 0.01,
	// 				"change": 0.00,
	// 				"volume": 1,
	// 				"open": 0.01,
	// 				"high": 0.01,
	// 				"low": 0.01,
	// 				"close": 0.01,
	// 				"bid": 0.0,
	// 				"ask": 0.01,
	// 				"underlying": "COIN",
	// 				"strike": 60.0,
	// 				"greeks": {
	// 					"delta": -1.9785489425E-6,
	// 					"gamma": 2.0795020742955597E-6,
	// 					"theta": 2.401870318626865E-13,
	// 					"vega": 2.0000001027016188E-5,
	// 					"rho": 0.0,
	// 					"phi": 0.0,
	// 					"bid_iv": 0.0,
	// 					"mid_iv": 0.0,
	// 					"ask_iv": 0.0,
	// 					"smv_vol": 45.378,
	// 					"updated_at": "2024-03-22 19:59:53"
	// 				},
	// 				"change_percentage": 0.00,
	// 				"average_volume": 0,
	// 				"last_volume": 1,
	// 				"trade_date": 1711114212085,
	// 				"prevclose": 0.01,
	// 				"week_52_high": 0.0,
	// 				"week_52_low": 0.0,
	// 				"bidsize": 0,
	// 				"bidexch": null,
	// 				"bid_date": 1711108801000,
	// 				"asksize": 39,
	// 				"askexch": "X",
	// 				"ask_date": 1711114212000,
	// 				"open_interest": 154,
	// 				"contract_size": 100,
	// 				"expiration_date": "2024-03-28",
	// 				"expiration_type": "quarterlys",
	// 				"option_type": "put",
	// 				"root_symbol": "COIN"
	// 			},
	// 			"unmatched_symbols": {
	// 				"symbol": [
	// 					"COIN240322C00285000",
	// 					"COIN240315C00262500",
	// 					"COIN240315C00262500"
	// 				]
	// 			}
	// 		}
	// 	}
	// `)

	input := []byte(`
		{
			"quotes": {
				"quote": {
					"symbol": "COIN240328P00060000",
					"description": "COIN Mar 28 2024 $60.00 Put",
					"exch": "Z",
					"type": "option",
					"last": 0.01,
					"change": 0.00,
					"volume": 1,
					"open": 0.01,
					"high": 0.01,
					"low": 0.01,
					"close": 0.01,
					"bid": 0.0,
					"ask": 0.01,
					"underlying": "COIN",
					"strike": 60.0,
					"greeks": {
						"delta": -1.9785489425E-6,
						"gamma": 2.0795020742955597E-6,
						"theta": 2.401870318626865E-13,
						"vega": 2.0000001027016188E-5,
						"rho": 0.0,
						"phi": 0.0,
						"bid_iv": 0.0,
						"mid_iv": 0.0,
						"ask_iv": 0.0,
						"smv_vol": 45.378,
						"updated_at": "2024-03-22 19:59:53"
					},
					"change_percentage": 0.00,
					"average_volume": 0,
					"last_volume": 1,
					"trade_date": 1711114212085,
					"prevclose": 0.01,
					"week_52_high": 0.0,
					"week_52_low": 0.0,
					"bidsize": 0,
					"bidexch": null,
					"bid_date": 1711108801000,
					"asksize": 39,
					"askexch": "X",
					"ask_date": 1711114212000,
					"open_interest": 154,
					"contract_size": 100,
					"expiration_date": "2024-03-28",
					"expiration_type": "quarterlys",
					"option_type": "put",
					"root_symbol": "COIN"
				}
			}
		}
	`)

	// input := []byte(`{
	// 	"quotes": {
	// 		"unmatched_symbols": {
	// 			"symbol": [
	// 				"COIN240322C00285000",
	// 				"COIN240315C00262500",
	// 				"COIN240315C00262500"
	// 			]
	// 		}
	// 	}
	// }`)

	var optionQuotes eventmodels.OptionQuotesDTO
	if err := json.Unmarshal(input, &optionQuotes); err != nil {
		fmt.Println("Error decoding JSON: ", err)
	}

	quotes, err := optionQuotes.ToModel()
	if err != nil {
		fmt.Println("Error converting dto to model: ", err)
	}

	fmt.Println(quotes)
	fmt.Printf("quotes len: %d\n", len(quotes))
}
