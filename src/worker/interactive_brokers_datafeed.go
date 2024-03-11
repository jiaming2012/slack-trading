package worker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
)

func IBSubscribe(conId string) []byte {
	return []byte(fmt.Sprintf(`smd+%s+{"fields":["31"]}`, conId))
}

// todo: refactor this to be compatible with Coinbase datafeed
func IBConnect(urlStr string, conId string) (*websocket.Conn, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	log.Infof("connecting to %s", u.String())

	// Create a custom Dialer with TLS confirguration to allow connecting to localhost
	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	c, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	if c == nil {
		return nil, fmt.Errorf("interactive brokers: failed to connect to websocket server: connection is nil")
	}

	time.Sleep(5 * time.Second)

	payload := IBSubscribe(conId)

	if err := c.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		return nil, fmt.Errorf("interactive brokers: connect: failed to write message: %v, using payload %s", err, payload)
	}

	return c, nil
}

type IBTickDTO struct {
	Price float64
}

type IBTickInfo struct {
	ConnID    string
	ServerURL string
}

type IBIncomingMessage struct {
	Topic string `json:"topic"`
}

type IBIncomingTickDTO struct {
	Topic     string `json:"topic"`
	TimeEpoch int    `json:"_updated"`
	Price     string `json:"31"`
	ConId     int    `json:"conid"`
}

func (t IBIncomingTickDTO) ToIBIncomingTick() (IBIncomingTick, error) {
	var symbol string

	// todo: fetch from config
	switch t.ConId {
	case 212921504:
		symbol = "CL"
	default:
		return IBIncomingTick{}, fmt.Errorf("ToIBIncomingTick: invalid conid: %d", t.ConId)
	}

	// convert time in epoch to time.Time
	milliseconds := int64(t.TimeEpoch)
	seconds := milliseconds / 1000
	nanoseconds := (milliseconds % 1000) * int64(time.Millisecond)
	ts := time.Unix(seconds, nanoseconds)

	// convert price to float64
	price, err := strconv.ParseFloat(t.Price, 64)
	if err != nil {
		return IBIncomingTick{}, fmt.Errorf("ToIBIncomingTick: failed to parse price: %w", err)
	}

	return IBIncomingTick{
		Symbol:    symbol,
		Timestamp: ts,
		Price:     price,
	}, nil
}

type IBIncomingTick struct {
	Symbol    string
	Timestamp time.Time
	Price     float64
}

type IBSMDMessage struct {
	Topic string `json:"topic"`
	Error string `json:"error"`
	Code  int    `json:"code"`
}

func IBTickListener(ctx context.Context, info IBTickInfo, ch chan IBTickDTO, c *websocket.Conn) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping IBTickListener")
				return
			default:
				// Read from the websocket
				c.SetReadDeadline(time.Now().UTC().Add(30 * time.Second))
				_, message, err := c.ReadMessage()
				if err != nil {
					log.Errorf("ReadMessage(): %v", err)

					// Reconnect
					newConn, newErr := IBConnect(info.ServerURL, info.ConnID)
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
				var dto IBIncomingTickDTO
				if err := json.Unmarshal(message, &dto); err != nil {
					log.Errorf("IBTickListener: failed to unmarshal message: %v", err)
					continue
				}

				// discard unknown messages
				if dto.Topic == "" {
					log.Warnf("IBTickListener: unknown message: %v", string(message))
					continue
				}

				if dto.Topic == "smd" {
					// parse error message
					var errMessage IBSMDMessage
					if err := json.Unmarshal(message, &errMessage); err != nil {
						log.Errorf("IBTickListener: failed to unmarshal error message: %v", err)
						continue
					}

					// ignore not authenticated messages
					log.Errorf("IBTickListener: smd code %d: %s", errMessage.Code, errMessage.Error)

					continue
				}

				// ignore system messages
				if dto.Topic == "system" {
					continue
				}

				if len(dto.Topic) < 3 || dto.Topic[:3] != "smd" {
					log.Warnf("IBTickListener: unknown topic: %v", dto.Topic)
					continue
				}

				// convert dto to IBIncomingTick
				tick, err := dto.ToIBIncomingTick()
				if err != nil {
					log.Errorf("IBTickListener: failed to convert dto to tick: %v", err)
					continue
				}

				eventpubsub.PublishEventResult("IBTickListener.worker", eventpubsub.NewTickEvent, eventmodels.NewTick(
					tick.Timestamp,
					tick.Price,
					eventmodels.IBDatafeed,
				))
			}
		}
	}()

	wg.Wait()
}
