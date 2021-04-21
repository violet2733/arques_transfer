package worker

import (
	"context"
	"fmt"
	"time"

	"arques.com/transfer/config"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type (
	WsTransferLog struct {
		Rds    *redis.Client
		ctx    context.Context
		Logger *log.Entry
		ws     *websocket.Conn
	}
)

func newWsTransferLog(ws *websocket.Conn) *WsTransferLog {
	c := &WsTransferLog{}

	c.ctx = context.Background()
	c.Rds = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Get().Redis.Url, config.Get().Redis.Port),
		Password: "",
		DB:       0,
	})

	c.Logger = log.WithFields(log.Fields{
		"workType": "WsTransferLog",
	})

	c.ws = ws

	return c
}

func ConnectionWsTransfer(ws *websocket.Conn) *WsTransferLog {
	c := newWsTransferLog(ws)
	return c
}

func (s *WsTransferLog) Subscribe() {
	pubsub := s.Rds.Subscribe(s.ctx, "gopax2binance")
	defer pubsub.Close()

	for {
		msgi, err := pubsub.ReceiveTimeout(s.ctx, time.Second)
		if err != nil {
			break
		}

		switch msg := msgi.(type) {

		case *redis.Message:
			err := s.ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
			if err != nil {
				s.Logger.WithError(err).Error(err)
			}
		}

	}
}
