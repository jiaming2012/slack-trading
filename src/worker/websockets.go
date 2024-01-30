package worker

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func connect() (*websocket.Conn, error) {
	// todo: remove fixed url
	u := url.URL{Scheme: "wss", Host: "advanced-trade-ws.coinbase.com", Path: "/"}
	log.Infof("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	if c == nil {
		return nil, fmt.Errorf("coinbase: failed to connect to websocket server: connection is nil")
	}

	payload := Subscribe()

	if err := c.WriteJSON(payload); err != nil {
		return nil, fmt.Errorf("coinbase: connect: failed to write json: %v, using payload %v", err, payload)
	}

	return c, nil
}

func WsTick(ctx context.Context, ch chan CoinbaseDTO) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	c, ConnErr := connect()
	if ConnErr != nil {
		log.Fatal("coinbase: initial connect failed:", ConnErr)
	}

	defer c.Close()

	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Read from the websocket
				c.SetReadDeadline(time.Now().UTC().Add(30 * time.Second))
				_, message, err := c.ReadMessage()

				if err != nil {
					log.Errorf("ReadMessage(): %v", err)

					// Reconnect
					newConn, newErr := connect()
					if newErr != nil {
						log.Errorf("failed to recconnect: %v", newErr)
						continue
					}

					if e := c.Close(); e != nil {
						log.Errorf("error closing old connection: %v", e)
					}

					c = newConn
					continue
				}

				// Unmarshal the message
				var update CoinbaseDTO
				err = json.Unmarshal(message, &update)
				if err != nil {
					log.Errorf("failed to unmarshal json: %v", err)
					continue
				}

				if update.Channel == "ticker" || update.Channel == "ticker_batch" {
					//if len(update.Events) > 0 {
					//	log.Println(len(update.Events), update.Events[0].Type)
					//}

					//log.Println(len(update.Events[0].Tickers), update.Events[0].Tickers[0].Type, update.Events[0].Tickers[0].ExecutedPrice)
					//log.Println(len(update.Events[0].Tickers), update.Events[0].Tickers[0].Volume24High)

					ch <- update
				}
			}
		}
	}()

	wg.Wait()
}

type CoinbaseTickerDTO struct {
	Type         string `json:"type"`
	ProductID    string `json:"product_id"`
	Price        string `json:"price"`
	Volume24High string `json:"volume_24_h"`
}

type CoinbaseEventDTO struct {
	Type    string              `json:"type"`
	Tickers []CoinbaseTickerDTO `json:"tickers"`
}

// todo: move to models
type CoinbaseDTO struct {
	Channel        string             `json:"channel"`
	ClientID       string             `json:"client_id"`
	Timestamp      time.Time          `json:"timestamp"`
	SequenceNumber int                `json:"sequence_num"`
	Events         []CoinbaseEventDTO `json:"events"`
}

func Subscribe() *WsSub {
	// todo: make this secret
	const secret = "s2RceoHWEaLYxnaeOUm2tpmNLsELkaGy"
	key := []byte(secret)

	//todo: multiple productIDs and level2
	productIDs := []string{"BTC-USD"}
	channel := "ticker_batch"

	ts := strconv.Itoa(int(time.Now().UTC().Unix()))
	strToSign := fmt.Sprintf("%s%s%s", ts, channel, strings.Join(productIDs, ","))

	h := hmac.New(sha256.New, key)
	h.Write([]byte(strToSign))

	return &WsSub{
		Type:       "subscribe",
		Channel:    channel,
		ApiKey:     "UPveTLyBzHNzsXRw",
		ProductIDs: productIDs,
		Timestamp:  ts,
		Signature:  hex.EncodeToString(h.Sum(nil)),
	}
}

//1680318106
type WsSub struct {
	Type       string   `json:"type"`
	Channel    string   `json:"channel"`
	ApiKey     string   `json:"api_key"`
	ProductIDs []string `json:"product_ids"`
	Signature  string   `json:"signature"`
	Timestamp  string   `json:"timestamp"`
}
