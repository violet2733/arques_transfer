package gopax

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// WsHandler handle raw websocket message
type WsHandler func(message []byte)

// ErrHandler handles errors
type ErrHandler func(err error)

// WsConfig webservice configuration
type WsConfig struct {
	Endpoint       string
	PublicRequest  *WsPublicRequest
	PrivateRequest *WsPrivateRequest
	Logger         *log.Entry
}

func newWsConfig(endpoint string) *WsConfig {
	w := &WsConfig{}

	w.Endpoint = endpoint
	w.Logger = log.WithFields(log.Fields{
		"workType": "gopax",
	})

	return w
}

var wsServe = func(cfg *WsConfig, handler WsHandler, errHandler ErrHandler) (doneC, stopC chan struct{}, err error) {
	websocket.DefaultDialer.HandshakeTimeout = 10 * time.Second
	c, _, err := websocket.DefaultDialer.Dial(cfg.Endpoint, nil)
	if err != nil {
		//log.Logger.WithError(err).Error("gopax websocket can not connected!")
		cfg.Logger.WithError(err).Error("gopax websocket can not connected!")
		return nil, nil, err
	}
	doneC = make(chan struct{})
	stopC = make(chan struct{})
	go func() {
		// This function will exit either on error from
		// websocket.Conn.ReadMessage or when the stopC channel is
		// closed by the client.
		defer close(doneC)
		if WebsocketKeepalive {
			keepAlive(c, WebsocketTimeout)
		}
		// Wait for the stopC channel to be closed.  We do that in a
		// separate goroutine because ReadMessage is a blocking
		// operation.
		silent := false
		go func() {
			select {
			case <-stopC:
				silent = true
			case <-doneC:
			}
			c.Close()
		}()

		if cfg.PublicRequest != nil {
			jsonByte, _ := json.Marshal(cfg.PublicRequest)
			fmt.Println(string(jsonByte))
			c.WriteMessage(websocket.TextMessage, jsonByte)
		}

		if cfg.PrivateRequest != nil {
			jsonByte, _ := json.Marshal(cfg.PrivateRequest)
			fmt.Println(string(jsonByte))
			c.WriteMessage(websocket.TextMessage, jsonByte)
		}

		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				if !silent {
					errHandler(err)
				}
				return
			}
			rawResponse := string(message)
			// fmt.Println(rawResponse)
			if strings.HasPrefix(rawResponse, "\"primus::ping::") { // 큰따옴표 유념 요망
				c.WriteMessage(websocket.TextMessage,
					[]byte("\"primus::pong::"+rawResponse[15:]))
			} else {
				handler(message)
			}

		}
	}()
	return
}

func keepAlive(c *websocket.Conn, timeout time.Duration) {
	ticker := time.NewTicker(timeout)

	lastResponse := time.Now()
	c.SetPongHandler(func(msg string) error {
		lastResponse = time.Now()
		return nil
	})

	go func() {
		defer ticker.Stop()
		for {
			deadline := time.Now().Add(10 * time.Second)
			err := c.WriteControl(websocket.PingMessage, []byte{}, deadline)
			if err != nil {
				return
			}
			<-ticker.C
			if time.Since(lastResponse) > timeout {
				c.Close()
				return
			}
		}
	}()
}
