package worker

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

func connect() (*websocket.Conn, error) {
	u := url.URL{Scheme: "wss", Host: "advanced-trade-ws.coinbase.com", Path: "/"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	sub := Subscribe()

	c.WriteJSON(sub)

	return c, nil
}

func WsTick(ch chan CoinbaseDTO) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	c, ConnErr := connect()
	if ConnErr != nil {
		log.Fatal("initial connect failed:", ConnErr)
	}

	defer c.Close()

	go func() {
		for {
			c.SetReadDeadline(time.Now().Add(30 * time.Second))
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
			}

			var update CoinbaseDTO

			json.Unmarshal(message, &update)
			if update.Channel == "ticker" {
				//log.Println(len(update.Events), update.Events[0].Type)
				//log.Println(len(update.Events[0].Tickers), update.Events[0].Tickers[0].Type, update.Events[0].Tickers[0].Price)
				//log.Println(len(update.Events[0].Tickers), update.Events[0].Tickers[0].Volume24High)

				ch <- update
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

type CoinbaseDTO struct {
	Channel        string             `json:"channel"`
	ClientID       string             `json:"client_id"`
	Timestamp      time.Time          `json:"timestamp"`
	SequenceNumber int                `json:"sequence_num"`
	Events         []CoinbaseEventDTO `json:"eventmodels"`
}

func Subscribe() *WsSub {
	const secret = "s2RceoHWEaLYxnaeOUm2tpmNLsELkaGy"
	key := []byte(secret)
	productIDs := []string{"BTC-USD"}
	channel := "ticker"

	ts := strconv.Itoa(int(time.Now().Unix()))
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
