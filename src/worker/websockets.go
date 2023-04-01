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

//var addr = flag.String("addr", "localhost:8080", "http service address")

func WsTest(ch chan CoinbaseDTO) {
	fmt.Println("ws test")
	wg := sync.WaitGroup{}
	wg.Add(1)

	u := url.URL{Scheme: "wss", Host: "advanced-trade-ws.coinbase.com", Path: "/"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Error(err)
				return
			}

			var update CoinbaseDTO
			log.Printf("msg: %s", message)
			json.Unmarshal(message, &update)
			log.Printf("recv: %v", update)
			if update.Channel == "ticker" {
				log.Println(len(update.Events), update.Events[0].Type)
				log.Println(len(update.Events[0].Tickers), update.Events[0].Tickers[0].Type, update.Events[0].Tickers[0].Price)
				log.Println(len(update.Events[0].Tickers), update.Events[0].Tickers[0].Volume24High)

				ch <- update
			}
		}
	}()

	fmt.Println(time.Now().Unix())
	sub := Subscribe()
	//jsonSub, err := json.Marshal(sub)
	//if err != nil {
	//	log.Error(err)
	//}

	c.WriteJSON(sub)

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
	Events         []CoinbaseEventDTO `json:"events"`
}

/*
strToSign:  1680318126tickerBTC-USD
sig:  7585ccbd9d4297d3176c3799c80350c001018e1051780837b2f7fcc4c042d915
*/

func Subscribe() *WsSub {
	const secret = "s2RceoHWEaLYxnaeOUm2tpmNLsELkaGy"
	key := []byte(secret)
	productIDs := []string{"BTC-USD"}
	channel := "ticker"

	ts := strconv.Itoa(int(time.Now().Unix()))
	//ts := "1680318126"
	strToSign := fmt.Sprintf("%s%s%s", ts, channel, strings.Join(productIDs, ","))
	//fmt.Println("sS: ", strToSign)

	h := hmac.New(sha256.New, key)
	h.Write([]byte(strToSign))
	//sig :=
	//fmt.Println(sig)

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
