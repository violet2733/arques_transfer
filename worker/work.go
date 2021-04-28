package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"arques.com/transfer/binancecustom"
	"arques.com/transfer/common"
	"arques.com/transfer/config"
	"arques.com/transfer/fixer"
	"arques.com/transfer/gopax"
	"arques.com/transfer/model"
	"arques.com/transfer/slack"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo"
	log "github.com/sirupsen/logrus"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
)

type (
	// Worker 메인 struct
	GimchiClient struct {
		GoPaxClient         *GoPaxClient
		BinanceSpotClient   *BinanceSpotClient
		BinanceFutureClient *BinanceFutureClient
		FixerClient         *FixerClient
		SlackClient         *SlackClient
		Rds                 *redis.Client
		ctx                 context.Context
		Logger              *log.Entry
		Data                *GimchiData
		WorkInfo            *model.TransferWorkInfo
		WorkStep            int
		BinanceBalance      *BinanceBalance
	}

	// 환율 정보를 위한 Fixer Client Struct
	FixerClient struct {
		ApiKey  string
		BaseURL string
		Client  *fixer.Client
		Logger  *log.Entry
	}

	// GoPax 접속에 필요한 Client
	GoPaxClient struct {
		ApiKey                  string
		SecretKey               string
		Client                  *gopax.Client
		Symbol                  string
		wg                      sync.WaitGroup
		UserDataTradeStopC      chan struct{}
		UserDataOrderBookStopC  chan struct{}
		UserDataOrdersStopC     chan struct{}
		UserDataUserTradesStopC chan struct{}
		pingTicker              *time.Ticker
		keepAliveTicker         *time.Ticker
		Logger                  *log.Entry
		OrderBook               *gopax.WsPublicOrderBookEvent
	}

	// Binance Spot 접속에 필요한 Client
	BinanceSpotClient struct {
		ApiKey                  string
		SecretKey               string
		Client                  *binance.Client
		Symbol                  string
		wg                      sync.WaitGroup
		UserDataTradeStopC      chan struct{}
		UserDataOrderBookStopC  chan struct{}
		UserDataUserTradesStopC chan struct{}
		pingTicker              *time.Ticker
		keepAliveTicker         *time.Ticker
		Logger                  *log.Entry
		OrderBook               *model.BinanceSpotOrderBook
	}

	// Binance Future 접속에 필요한 Client
	BinanceFutureClient struct {
		ApiKey                  string
		SecretKey               string
		Client                  *futures.Client
		Symbol                  string
		wg                      sync.WaitGroup
		UserDataTradeStopC      chan struct{}
		UserDataOrderBookStopC  chan struct{}
		UserDataUserTradesStopC chan struct{}
		pingTicker              *time.Ticker
		keepAliveTicker         *time.Ticker
		Logger                  *log.Entry
		OrderBook               *model.BinanceFutureOrderBook
	}

	// Slack Noti Struct
	SlackClient struct {
		BaseURL string
		Channel string
		Client  *slack.Client
	}

	// 김프 데이터
	GimchiData struct {
		GopaxPrice   float64
		BinancePrice float64
		EUR          float64
		KRW          float64
		USD          float64
		Rate         float64
	}

	// Noti Message Struct
	Message struct {
		Title string
		Msg   interface{}
	}

	// Process struct {
	// 	Step int
	// 	Idx  int
	// }

	// Binance Balance Info Struct
	BinanceBalance struct {
		Spot   *binance.Balance
		Future *futures.AccountPosition
	}
)

type ExchangeType string

const (
	ExchangeTypeGoPax         ExchangeType = "Gopax"
	ExchangeTypeBinanceSpot   ExchangeType = "BinanceSpot"
	ExchangeTypeBinanceFuture ExchangeType = "BinanceFuture"
)

// GimchiClient init
func newClient() *GimchiClient {
	c := &GimchiClient{}
	c.ctx = context.Background()

	// 클라이언트 로거 설정
	c.Logger = log.WithFields(log.Fields{
		"workType": "gimchi",
	})

	// 레디스 클라이언트 생성
	c.Rds = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Get().Redis.Url, config.Get().Redis.Port),
		Password: "",
		DB:       0,
	})

	// app.json 파일 읽어서 WorkInfo 생성
	workinfo, err := c.getWorkJson()
	if err != nil {
		c.Logger.WithError(err).Error("Failed to read app.json file.")
		return nil
	}
	c.WorkInfo = workinfo

	// 프로세스에 필요한 데이터 선언
	c.Data = &GimchiData{
		GopaxPrice:   0,
		BinancePrice: 0,
		EUR:          0,
		KRW:          0,
		USD:          0,
		Rate:         0,
	}

	// 각 Client 생성하기
	c.GoPaxClient = newGoPaxClient(c.WorkInfo.WorkSymbol)
	c.BinanceSpotClient = newBinanceSpotClient(c.WorkInfo.WorkSymbol)
	c.BinanceFutureClient = newBinanceFutureClient(c.WorkInfo.WorkSymbol)
	c.FixerClient = newFixerClient()
	c.SlackClient = newSlackClient()

	// WorkStep 은 초기 0
	c.WorkStep = 0

	// Fixer 를 통한 화폐 환율 조회
	if err := c.GetExchangeRateData(); err != nil {
		c.Logger.WithError(err).Error("Failed to get exchange money rate")
	}

	return c
}

// Gopax Client 생성
func newGoPaxClient(symbol string) *GoPaxClient {
	c := &GoPaxClient{}
	c.ApiKey = config.Get().Gopax.ApiKey
	c.SecretKey = config.Get().Gopax.SecretKey
	c.Client = gopax.NewClient(c.ApiKey, c.SecretKey)

	c.Symbol = fmt.Sprintf("%s-KRW", symbol)

	c.Logger = log.WithFields(log.Fields{
		"workType": "gopax",
	})

	return c
}

// Binance Spot Client 생성
func newBinanceSpotClient(symbol string) *BinanceSpotClient {
	c := &BinanceSpotClient{}
	c.ApiKey = config.Get().Binance.ApiKey
	c.SecretKey = config.Get().Binance.SecretKey
	c.Client = binance.NewClient(c.ApiKey, c.SecretKey)

	c.Symbol = fmt.Sprintf("%sUSDT", symbol)

	c.Logger = log.WithFields(log.Fields{
		"workType": "gopax",
	})

	return c
}

// Binance Future Client 생성
func newBinanceFutureClient(symbol string) *BinanceFutureClient {
	c := &BinanceFutureClient{}
	c.ApiKey = config.Get().Binance.ApiKey
	c.SecretKey = config.Get().Binance.SecretKey
	c.Client = futures.NewClient(c.ApiKey, c.SecretKey)

	c.Symbol = fmt.Sprintf("%sUSDT", symbol)

	c.Logger = log.WithFields(log.Fields{
		"workType": "gopax",
	})

	return c
}

// Fixer Client 생성
func newFixerClient() *FixerClient {
	c := &FixerClient{}
	c.ApiKey = config.Get().Fixer.AccessKey
	c.BaseURL = config.Get().Fixer.Url
	c.Client = fixer.NewClient(c.BaseURL, c.ApiKey)
	c.Logger = log.WithFields(log.Fields{
		"workType": "gopax",
	})

	return c
}

// Slack Client 생성
func newSlackClient() *SlackClient {
	c := &SlackClient{}
	c.BaseURL = config.Get().Slack.Url
	c.Channel = "#noti-gopax2binance"

	c.Client = slack.NewClient(c.BaseURL, c.Channel)

	return c
}

// app.json 읽어서 workInfo 조회
func GetWorkInfo(c echo.Context) error {
	jsonFile, err := os.Open("app.json")
	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var info model.TransferWorkInfo

	json.Unmarshal([]byte(byteValue), &info)

	return c.JSON(http.StatusOK, info)
}

// 임시로 생성한 메소드 ( 초기 웹서버 실행시 자동으로 실행하려고 만든 메소드 )
func ConnectionGoPax() {
	c := newClient()
	c.Logger.Info("GoPax Start!!")

}

// 김프 계산
func GimchiCalculator() (data *GimchiData, err error) {
	c := newClient()
	data, err = c.GetGimchiPremium()
	if err != nil {
		return nil, err
	}

	return data, err
}

func CheckGopaxBalance() (data []*gopax.Balance, err error) {
	c := newClient()

	data, err = c.getGopaxAccountInfo()
	if err != nil {
		c.Logger.WithError(err).Error("CheckGopaxBalance")
	}
	return data, nil
}

func GetBinanceFutureOrderHistory() (data []*futures.AccountTrade, err error) {
	c := newClient()

	data, err = c.BinanceFutureClient.Client.NewListAccountTradeService().Symbol(c.BinanceFutureClient.Symbol).Limit(100).Do(c.ctx)
	if err != nil {
		c.Logger.WithError(err).Error("CheckBinanceFutureOrderHistory")
		return nil, err
	}
	return data, nil
}

// Binance Spot Balance 조회
func CheckBinanceSpotBalance() (data *binance.Balance, err error) {
	c := newClient()
	data, err = c.getBinanceSpotAccountInfo()
	if err != nil {
		c.Logger.WithError(err).Error("CheckBinanceSpotBalance")
		return nil, err
	}

	return data, nil
}

func CheckBinanceFutureBalance() (data *futures.AccountPosition, err error) {
	c := newClient()
	data, err = c.getBinanceFutureAccountInfo()
	if err != nil {
		c.Logger.WithError(err).Error("CheckBinanceFutureBalance")
		return nil, err
	}

	return data, nil
}

// GoPax2Binance First Step Start
func StartGoPax2BinanceFirstStep() {
	c := newClient()
	c.Logger.Info("GoPax First Step Start!!")

	// 프로세스 시작
	c.SendMessage("GoPax2Binance FirstStep Start!", c.WorkInfo)

	// GoPax 의 최근 거래내역 조회
	if err := c.StartGoPaxTradeStream(); err != nil {
		c.Logger.WithError(err).Error("Failed to start Trade by GoPax.")
	}

	// Binance 의 최근 거래내역 조회
	if err := c.StartBinanceSpotTradeStream(); err != nil {
		c.Logger.WithError(err).Error("Failed to start Trade by Binance.")
	}
}

// GoPax2Binance Second Step Start
func StartGoPax2BinanceSecondStep() {
	c := newClient()

	c.Logger.Info("GoPax Second Step Start!!")

	// 프로세스 시작
	c.SendMessage("GoPax2Binance SecondStep Start!", c.WorkInfo)

	// Binance Spot Sell and Binance Future Short Position Close
	c.BinanceSpotSellAndBinanceFuturePositionClose()
}

// Fixer 환율 값 가져오기
func (c *GimchiClient) GetExchangeRateData() error {
	data, err := c.FixerClient.Client.GetExchangeRateData()
	if err != nil {
		return err
	}

	if data != nil {
		c.Data.EUR = data.Countries.EUR
		c.Data.KRW = data.Countries.KRW
		c.Data.USD = data.Countries.USD
	}

	return nil
}

// API 로 김프 계산
func (c *GimchiClient) GetGimchiPremium() (data *GimchiData, err error) {
	gopaxData, err := c.GoPaxClient.Client.NewGetTickerService().Symbol(c.GoPaxClient.Symbol).Do(c.ctx)
	if err != nil {
		c.Logger.WithError(err).Error("Failed to get gopax tick data")
		return nil, err
	}
	c.Data.GopaxPrice = float64(int(gopaxData.Price))

	binanceData, err := c.BinanceFutureClient.Client.NewPremiumIndexService().Symbol(c.BinanceFutureClient.Symbol).Do(c.ctx)

	if err != nil {
		c.Logger.WithError(err).Error("Failed to get binance Mark Price")
		return nil, err
	}
	c.Data.BinancePrice, err = strconv.ParseFloat(binanceData.MarkPrice, 8)
	if err != nil {
		c.Logger.WithError(err).Error("Failed to convert float to string")
		return nil, err
	}

	c.Data.getRate()

	return c.Data, nil
}

// GoPax Public Trade Data Stream
func (c *GimchiClient) StartGoPaxTradeStream() error {
	errHandler := func(err error) {
		c.Logger.WithError(err).Error("Gopax Trade Stream Error!")
		c.RestartGoPaxTradeStream()
	}

	go func() {
		doneC, stopC, err := gopax.WsPublicTradeServe(c.GoPaxClient.Symbol, c.messageGoPaxTradeHandler, errHandler)
		if err != nil {
			c.Logger.WithError(err).Error("Failed Gopax Trade Stream data wss")
			return
		}
		c.GoPaxClient.UserDataTradeStopC = stopC
		<-doneC
		c.Logger.Info("Gopax Trade Stream stream closed.")
	}()

	return nil
}

func (c *GimchiClient) RestartGoPaxTradeStream() error {
	c.GoPaxClient.keepAliveTicker.Stop()
	c.StopGoPaxTradeStream()
	return c.StartGoPaxTradeStream()
}

func (c *GimchiClient) StopGoPaxTradeStream() {
	c.GoPaxClient.UserDataTradeStopC <- struct{}{}
}

// GoPax Public Order Book Stream
func (c *GimchiClient) StartGoPaxOrderBookStream() error {
	errHandler := func(err error) {
		c.Logger.WithError(err).Error("Gopax OrderBook Stream Error!")
		c.RestartGoPaxOrderBookStream()
	}

	go func() {
		doneC, stopC, err := gopax.WsPublicOrderBookServe(c.GoPaxClient.Symbol, c.messageGoPaxOrderBookHandler, errHandler)
		if err != nil {
			c.Logger.WithError(err).Error("Failed Gopax OrderBook Stream data wss")
			return
		}
		c.GoPaxClient.UserDataOrderBookStopC = stopC
		<-doneC
		c.Logger.Info("Gopax OrderBook Stream stream closed.")
	}()

	return nil
}

func (c *GimchiClient) RestartGoPaxOrderBookStream() error {
	c.GoPaxClient.keepAliveTicker.Stop()
	c.StopGoPaxOrderBookStream()
	return c.StartGoPaxOrderBookStream()
}

func (c *GimchiClient) StopGoPaxOrderBookStream() {
	c.GoPaxClient.UserDataOrderBookStopC <- struct{}{}
}

// GoPax Private Orders stream
func (c *GimchiClient) StartGoPaxOrdersStream() error {
	errHandler := func(err error) {
		c.Logger.WithError(err).Error("Gopax Orders Stream Error!")
		c.RestartGoPaxOrdersStream()
	}

	go func() {
		doneC, stopC, err := gopax.WsPrivateOrdersServe(c.messageGoPaxOrdersHandler, errHandler)
		if err != nil {
			c.Logger.WithError(err).Error("Failed Gopax OrderBook Stream data wss")
			return
		}
		c.GoPaxClient.UserDataOrdersStopC = stopC
		<-doneC
		c.Logger.Info("Gopax OrderBook Stream stream closed.")
	}()

	return nil
}

func (c *GimchiClient) RestartGoPaxOrdersStream() error {
	c.GoPaxClient.keepAliveTicker.Stop()
	c.StopGoPaxOrdersStream()
	return c.StartGoPaxOrdersStream()
}

func (c *GimchiClient) StopGoPaxOrdersStream() {
	c.GoPaxClient.UserDataOrdersStopC <- struct{}{}
}

// GoPax Private Orders stream
func (c *GimchiClient) StartGoPaxUserTradesStream() error {
	errHandler := func(err error) {
		c.Logger.WithError(err).Error("Gopax Orders Stream Error!")
		c.RestartGoPaxUserTradesStream()
	}

	go func() {
		doneC, stopC, err := gopax.WsPrivateUserTradesServe(c.messageGoPaxUserTradesHandler, errHandler)
		if err != nil {
			c.Logger.WithError(err).Error("Failed Gopax OrderBook Stream data wss")
			return
		}
		c.GoPaxClient.UserDataUserTradesStopC = stopC
		<-doneC
		c.Logger.Info("Gopax OrderBook Stream stream closed.")
	}()

	return nil
}

func (c *GimchiClient) RestartGoPaxUserTradesStream() error {
	c.GoPaxClient.keepAliveTicker.Stop()
	c.StopGoPaxUserTradesStream()
	return c.StartGoPaxUserTradesStream()
}

func (c *GimchiClient) StopGoPaxUserTradesStream() {
	c.GoPaxClient.UserDataOrdersStopC <- struct{}{}
}

// Binance Spot Public Trade Data Stream
func (c *GimchiClient) StartBinanceSpotTradeStream() error {
	errHandler := func(err error) {
		c.Logger.WithError(err).Error("Stream Error!")
		c.RestartBinanceSpotTradeStream()
	}

	go func() {
		doneC, stopC, err := binance.WsAggTradeServe(c.BinanceSpotClient.Symbol, c.messageBinanceSpotTradeHandler, errHandler)
		if err != nil {
			c.Logger.WithError(err).Error("Failed public data wss")
			return
		}
		c.BinanceSpotClient.UserDataTradeStopC = stopC
		<-doneC
		c.Logger.Info("User data stream closed.")
	}()

	return nil
}

func (c *GimchiClient) RestartBinanceSpotTradeStream() error {
	c.BinanceSpotClient.keepAliveTicker.Stop()
	c.StopBinanceSpotTradeStream()
	return c.StartBinanceSpotTradeStream()
}

func (c *GimchiClient) StopBinanceSpotTradeStream() {
	c.BinanceSpotClient.UserDataTradeStopC <- struct{}{}
}

// Binance Spot Public Order Book Stream
func (c *GimchiClient) StartBinanceSpotOrderBookStream() error {

	c.SnapshotBinanceSpotOrderBook()

	errHandler := func(err error) {
		c.Logger.WithError(err).Error("Stream Error!")
		c.RestartBinanceSpotOrderBookStream()
	}

	go func() {
		doneC, stopC, err := binance.WsDepthServe(c.BinanceSpotClient.Symbol, c.messageBinanceSpotOrderBookHandler, errHandler)
		if err != nil {
			c.Logger.WithError(err).Error("Failed public data wss")
			return
		}
		c.BinanceSpotClient.UserDataOrderBookStopC = stopC
		<-doneC
		c.Logger.Info("User data stream closed.")
	}()

	return nil
}

func (c *GimchiClient) RestartBinanceSpotOrderBookStream() error {
	c.BinanceSpotClient.keepAliveTicker.Stop()
	c.StopBinanceSpotOrderBookStream()
	return c.StartBinanceSpotOrderBookStream()
}

func (c *GimchiClient) StopBinanceSpotOrderBookStream() {
	c.BinanceSpotClient.UserDataOrderBookStopC <- struct{}{}
}

// Binance Spot User Stream
func (c *GimchiClient) BinanceSpotListenKey() (string, error) {
	res, err := c.BinanceSpotClient.Client.NewStartUserStreamService().Do(c.ctx)
	if err != nil {
		c.Logger.WithError(err).Error("ListenKey failed.")
		return "", err
	}
	return res, nil
}

func (c *GimchiClient) BinanceSpotKeepAlive() error {
	err := c.BinanceSpotClient.Client.NewKeepaliveUserStreamService().Do(c.ctx)
	if err != nil {
		c.Logger.WithError(err).Error("KeepAlive failed.")
		c.RestartBinanceSpotUserStream()
	}
	return nil
}

func (c *GimchiClient) StartBinanceSpotUserStream() error {
	listenKey, err := c.BinanceSpotListenKey()
	if err != nil {
		return err
	}

	// ListenKey가 expire하는 것을 막기 위해 30분 마다 keepAlive를 전송한다.
	c.BinanceSpotClient.keepAliveTicker = time.NewTicker(time.Minute * 30)
	go func() {
		for range c.BinanceSpotClient.keepAliveTicker.C {
			c.BinanceSpotKeepAlive()
		}
	}()

	errHandler := func(err error) {
		c.Logger.WithError(err).Error("Stream error.")
		// 계속 실패하면 무한 루프처럼 돌 수도..
		c.RestartBinanceSpotUserStream()
	}

	go func() {
		// NOTE: KeepAlive 옵션을 켜도 웹소켓이 1시간 후 만료된다. 핑퐁밖에는 안 해주는 것 같아보임.
		binance.WebsocketKeepalive = true
		futures.WebsocketKeepalive = true

		doneC, stopC, err := binance.WsUserDataServe(listenKey, c.messageBinanceSpotUserTradeHandler, errHandler)
		if err != nil {
			c.Logger.WithError(err).Error("Failed to open user data ws.")
			return
		}
		c.BinanceSpotClient.UserDataUserTradesStopC = stopC
		<-doneC
		c.Logger.Info("User data stream closed.")
	}()

	return nil
}

func (c *GimchiClient) RestartBinanceSpotUserStream() error {
	c.BinanceSpotClient.keepAliveTicker.Stop()
	c.StopBinanceSpotUserStream()
	return c.StartBinanceSpotUserStream()
}

func (c *GimchiClient) StopBinanceSpotUserStream() {
	c.BinanceSpotClient.UserDataUserTradesStopC <- struct{}{}
}

// Binance Future Public Order Book Stream
func (c *GimchiClient) StartBinanceFutureOrderBookStream() error {

	c.SnapshotBinanceFutureOrderBook()

	errHandler := func(err error) {
		c.Logger.WithError(err).Error("Stream Error!")
		c.RestartBinanceFutureOrderBookStream()
	}

	go func() {
		doneC, stopC, err := futures.WsDiffDepthServe(c.BinanceSpotClient.Symbol, c.messageBinanceFutureOrderBookHandler, errHandler)
		if err != nil {
			c.Logger.WithError(err).Error("Failed public data wss")
			return
		}
		c.BinanceSpotClient.UserDataOrderBookStopC = stopC
		<-doneC
		c.Logger.Info("User data stream closed.")
	}()

	return nil
}

func (c *GimchiClient) RestartBinanceFutureOrderBookStream() error {
	c.BinanceFutureClient.keepAliveTicker.Stop()
	c.StopBinanceFutureOrderBookStream()
	return c.StartBinanceFutureOrderBookStream()
}

func (c *GimchiClient) StopBinanceFutureOrderBookStream() {
	c.BinanceFutureClient.UserDataOrderBookStopC <- struct{}{}
}

// Binance Future User Stream
func (c *GimchiClient) BinanceFutureListenKey() (string, error) {
	res, err := c.BinanceFutureClient.Client.NewStartUserStreamService().Do(c.ctx)
	if err != nil {
		c.Logger.WithError(err).Error("ListenKey failed.")
		return "", err
	}
	return res, nil
}

func (c *GimchiClient) BinanceFutureKeepAlive() error {
	err := c.BinanceFutureClient.Client.NewKeepaliveUserStreamService().Do(c.ctx)
	if err != nil {
		c.Logger.WithError(err).Error("KeepAlive failed.")
		c.RestartBinanceFutureUserStream()
	}
	return nil
}

func (c *GimchiClient) StartBinanceFutureUserStream() error {
	listenKey, err := c.BinanceFutureListenKey()
	if err != nil {
		return err
	}

	// ListenKey가 expire하는 것을 막기 위해 30분 마다 keepAlive를 전송한다.
	c.BinanceFutureClient.keepAliveTicker = time.NewTicker(time.Minute * 30)
	go func() {
		for range c.BinanceFutureClient.keepAliveTicker.C {
			c.BinanceFutureKeepAlive()
		}
	}()

	errHandler := func(err error) {
		c.Logger.WithError(err).Error("Stream error.")
		// 계속 실패하면 무한 루프처럼 돌 수도..
		c.RestartBinanceFutureUserStream()
	}

	go func() {
		// NOTE: KeepAlive 옵션을 켜도 웹소켓이 1시간 후 만료된다. 핑퐁밖에는 안 해주는 것 같아보임.
		binance.WebsocketKeepalive = true
		futures.WebsocketKeepalive = true

		doneC, stopC, err := binancecustom.WsFutureUserDataServe(listenKey, c.messageBinanceFutureUserTradeHandler, errHandler)
		// doneC, stopC, err := futures.WsFutureUserDataServe(listenKey, c.messageBinanceFutureUserTradeHandler, errHandler, interface{})
		if err != nil {
			c.Logger.WithError(err).Error("Failed to open user data ws.")
			return
		}
		c.BinanceFutureClient.UserDataUserTradesStopC = stopC
		<-doneC
		c.Logger.Info("User data stream closed.")
	}()

	return nil
}

func (c *GimchiClient) RestartBinanceFutureUserStream() error {
	c.BinanceFutureClient.keepAliveTicker.Stop()
	c.StopBinanceFutureUserStream()
	return c.StartBinanceFutureUserStream()
}

func (c *GimchiClient) StopBinanceFutureUserStream() {
	c.BinanceFutureClient.UserDataUserTradesStopC <- struct{}{}
}

// GoPax WebSocket Orderbook Handler
func (c *GimchiClient) messageGoPaxOrderBookHandler(message []byte) {
	c.GoPaxClient.wg.Wait()

	orderbook := new(gopax.WsPublicOrderBookEvent)
	if err := json.Unmarshal(message, orderbook); err != nil {
		c.Logger.WithError(err).Error("Failed to check data by Gopax Orderbook WebSocket data")
	}

	if orderbook.Name == "SubscribeToOrderBook" {
		c.GoPaxClient.OrderBook = orderbook
	} else if orderbook.Name == "OrderBookEvent" {
		if len(orderbook.Data.Ask) > 0 {

			for _, i := range orderbook.Data.Ask {
				idx, d := c.GoPaxClient.OrderBook.Data.Filter(gopax.SideTypeSell, i)
				if idx == -1 {
					c.GoPaxClient.OrderBook.Data.Ask = append(c.GoPaxClient.OrderBook.Data.Ask, i)
				} else {
					if d.Volume == 0 {
						c.GoPaxClient.OrderBook.Data.Remove(gopax.SideTypeSell, idx)
					}
				}
			}
		}

		if len(orderbook.Data.Bid) > 0 {
			for _, i := range orderbook.Data.Bid {
				idx, d := c.GoPaxClient.OrderBook.Data.Filter(gopax.SideTypeBuy, i)
				if idx == -1 {
					c.GoPaxClient.OrderBook.Data.Bid = append(c.GoPaxClient.OrderBook.Data.Bid, i)
				} else {
					if d.Volume == 0 {
						c.GoPaxClient.OrderBook.Data.Remove(gopax.SideTypeBuy, idx)
					}
				}
			}

		}

		sort.Slice(c.GoPaxClient.OrderBook.Data.Ask, func(i, j int) bool {
			return c.GoPaxClient.OrderBook.Data.Ask[i].Price < c.GoPaxClient.OrderBook.Data.Ask[j].Price
		})

		sort.Slice(c.GoPaxClient.OrderBook.Data.Bid, func(i, j int) bool {
			return c.GoPaxClient.OrderBook.Data.Bid[i].Price > c.GoPaxClient.OrderBook.Data.Bid[j].Price
		})

	}
}

// GoPax WebSocket Trade data Handler
func (c *GimchiClient) messageGoPaxTradeHandler(message []byte) {
	c.GoPaxClient.wg.Wait()

	trade := new(gopax.WsPublicTradeEvent)
	if err := json.Unmarshal(message, trade); err != nil {
		c.Logger.WithError(err).Error("Failed to check data by Gopax Trade Websocket data")
		return
	}

	if trade.Name == "PublicTradeEvent" {
		c.Data.GopaxPrice = float64(trade.Data.Price)

		if err := c.GetExchangeRateData(); err != nil {
			c.Logger.WithError(err).Error("Failed to get exchange money rate by Gopax Trade Websocket data")
		}

		c.Data.getRate()
		c.MonitorGimchiPremium()
	}

}

// GoPax WebSocket Order data Handler
func (c *GimchiClient) messageGoPaxOrdersHandler(message []byte) {
	c.GoPaxClient.wg.Wait()

	checker := new(gopax.WsPrivateEvent)
	if err := json.Unmarshal(message, checker); err != nil {
		c.Logger.WithError(err).Error("Failed to check data by Gopax Trade Websocket data")
		return
	}

	if checker.Name == "SubscribeToOrders" {
		response := new(gopax.WsPrivateOrdersResponseEvent)
		if err := json.Unmarshal(message, response); err != nil {
			c.Logger.WithError(err).Error("Failed to check data by Gopax Trade Websocket data")
			return
		}

	} else if checker.Name == "OrderEvent" {
		push := new(gopax.WsPrivateOrdersPushEvent)
		if err := json.Unmarshal(message, push); err != nil {
			c.Logger.WithError(err).Error("Failed to check data by Gopax Trade Websocket data")
			return
		}
	}
}

// GoPax WebSocket User Recent Trade data Handler
func (c *GimchiClient) messageGoPaxUserTradesHandler(message []byte) {
	c.GoPaxClient.wg.Wait()

	checker := new(gopax.WsPrivateEvent)
	if err := json.Unmarshal(message, checker); err != nil {
		c.Logger.WithError(err).Error("Failed to check data by Gopax Trade Websocket data")
		return
	}

	if checker.Name == "SubscribeToTrades" {
		response := new(gopax.WsPrivateTradesResponseEvent)
		if err := json.Unmarshal(message, response); err != nil {
			c.Logger.WithError(err).Error("Failed to check data by Gopax Trade Websocket data")
			return
		}
	} else if checker.Name == "TradeEvent" {
		push := new(gopax.WsPrivateTradesPushEvent)
		if err := json.Unmarshal(message, push); err != nil {
			c.Logger.WithError(err).Error("Failed to check data by Gopax Trade Websocket data")
			return
		}

		// fmt.Println(push)

		// First Step 에서 Gopax 체결이 완료 되면 완료처리 하고 Binance Future Short Position 주문 실행
		orderId := push.Data.OrderId

		i, openOrder, err := c.filterOrderWork(ExchangeTypeGoPax, orderId)
		if err != nil {
			c.Logger.WithError(err).Error("Invalid Gopax OrderID")
		} else {

			if c.WorkStep == 1 {

				openOrder.IsFinished = true
				openOrder.UpdatedAt = common.Now()
				c.WorkInfo.FirstStep.GoPaxWorks[i] = openOrder

				check := <-c.checkGoPaxOrder(int(orderId))
				if check {
					// Binance Future Order
					_, err := c.createBinanceFutureOrderAsync(i)
					if err != nil {
						c.Logger.WithError(err).Error("Failed to create order in binance future")
					}
				}

				// orderResult, err := c.GoPaxClient.Client.NewGetOrderService().OrderId(strconv.Itoa(int(orderId))).Do(c.ctx)
				// if err != nil {
				// 	c.Logger.WithError(err).Error("GoPax Order Result API Call Err")
				// } else {
				// 	// defer c.GoPaxClient.wg.Done()

				// 	if orderResult.Status == "completed" {
				// 		// Binance Future Order
				// 		err := c.createBinanceFutureOrder(i)
				// 		if err != nil {
				// 			c.Logger.WithError(err).Error("Failed to create order in binance future")
				// 		}
				// 	}
				// }

			}
		}
	}

}

// Binance WebSocket Trade data Handler
func (c *GimchiClient) messageBinanceSpotTradeHandler(data *binance.WsAggTradeEvent) {
	f, _ := strconv.ParseFloat(data.Price, 32)
	c.Data.BinancePrice = f

	if err := c.GetExchangeRateData(); err != nil {
		c.Logger.WithError(err).Error("Failed to get exchange money rate")
	}

	c.Data.getRate()
	c.MonitorGimchiPremium()
}

// Binance Spot WebSocket Orderbook data Handler
func (c *GimchiClient) messageBinanceSpotOrderBookHandler(data *binance.WsDepthEvent) {
	if data.UpdateID < c.BinanceSpotClient.OrderBook.Data.LastUpdateID {

	} else if data.FirstUpdateID <= c.BinanceSpotClient.OrderBook.Data.LastUpdateID+1 && data.UpdateID >= c.BinanceSpotClient.OrderBook.Data.LastUpdateID && c.BinanceSpotClient.OrderBook.ProcessID == 0 {
		c.setBinanceSpotOrderBook(data)
	} else if c.BinanceSpotClient.OrderBook.ProcessID == data.UpdateID {
		c.setBinanceSpotOrderBook(data)
	} else {
		c.SnapshotBinanceSpotOrderBook()
	}
}

// Binance Spot Orderbook 최초 1회 Snapshot 가져오기
func (c *GimchiClient) SnapshotBinanceSpotOrderBook() error {
	orderbook, err := c.BinanceSpotClient.Client.NewDepthService().Symbol(c.BinanceSpotClient.Symbol).Do(c.ctx)

	if err != nil {
		return err
	}
	c.BinanceSpotClient.OrderBook = &model.BinanceSpotOrderBook{}
	c.BinanceSpotClient.OrderBook.ProcessID = 0
	c.BinanceSpotClient.OrderBook.Data = orderbook

	return nil
}

// Binance Spot Orderbook WS 에서 받은 걸로 계산 처리
func (c *GimchiClient) setBinanceSpotOrderBook(data *binance.WsDepthEvent) {
	if len(data.Asks) > 0 {
		for _, i := range data.Asks {
			idx, d := c.BinanceSpotClient.OrderBook.BinanceSpotOrderBookFilterAsk(&i)
			if idx == -1 {
				c.BinanceSpotClient.OrderBook.Data.Asks = append(c.BinanceSpotClient.OrderBook.Data.Asks, i)
			} else {
				v, err := strconv.ParseFloat(d.Quantity, 1)
				if err != nil {
					c.Logger.WithError(err).Error("Error convert from string to float")
				}
				if v == 0 {
					c.BinanceSpotClient.OrderBook = c.BinanceSpotClient.OrderBook.BinanceSpotOrderBookRemoveAsk(idx)
				}
			}
		}
	}

	if len(data.Bids) > 0 {
		for _, i := range data.Bids {
			idx, d := c.BinanceSpotClient.OrderBook.BinanceSpotOrderBookFilterBid(&i)
			if idx == -1 {
				c.BinanceSpotClient.OrderBook.Data.Bids = append(c.BinanceSpotClient.OrderBook.Data.Bids, i)
			} else {
				v, err := strconv.ParseFloat(d.Quantity, 1)
				if err != nil {
					c.Logger.WithError(err).Error("Error convert from string to float")
				}
				if v == 0 {
					c.BinanceSpotClient.OrderBook = c.BinanceSpotClient.OrderBook.BinanceSpotOrderBookRemoveBid(idx)
				}
			}
		}
	}

	sort.Slice(c.BinanceSpotClient.OrderBook.Data.Asks, func(i, j int) bool {
		return c.BinanceSpotClient.OrderBook.Data.Asks[i].Price < c.BinanceSpotClient.OrderBook.Data.Asks[j].Price
	})

	sort.Slice(c.BinanceSpotClient.OrderBook.Data.Bids, func(i, j int) bool {
		return c.BinanceSpotClient.OrderBook.Data.Bids[i].Price > c.BinanceSpotClient.OrderBook.Data.Bids[j].Price
	})

	c.BinanceSpotClient.OrderBook.ProcessID = data.UpdateID
}

// Binance Spot WebSocket User Trade Data Handler
func (c *GimchiClient) messageBinanceSpotUserTradeHandler(message []byte) {
	// 주문 전송이 진행중인 경우 끝날때까지 기다린다.
	c.BinanceSpotClient.wg.Wait()

	eventTypeChecker := new(model.EventTypeChecker)
	if err := json.Unmarshal(message, eventTypeChecker); err != nil {
		c.Logger.WithError(err).WithField("msg", string(message)).Error("Failed to check event type.")
		return
	}

	switch eventTypeChecker.EventType {
	case "balanceUpdate":
		return
	case "outboundAccountPosition":
		return
	case "listStatus":
		return
	case "listenKeyExpired":
		if err := c.RestartBinanceSpotUserStream(); err != nil {
			c.Logger.WithError(err).Error("Failed to restart userstream")
			return
		}
	case "executionReport":
		orderResult := new(model.SpotOrderResult)
		if err := json.Unmarshal(message, orderResult); err != nil {
			c.Logger.WithError(err).WithField("msg", string(message)).Error("Failed to unmarshal orderresult update.")
			return
		}
		c.Logger.WithField("result", orderResult).Info()
		if err := c.handleBinanceOrderTradeResult(ExchangeTypeBinanceSpot, orderResult); err != nil {
			c.Logger.WithField("result", orderResult).WithError(err).Error("Failed to handle trade update")
			return
		}
	default:
		c.Logger.WithField("msg", string(message)).Error("Message not handled.")
	}
}

// Binance Future WebSocket Orderbook data Handler
func (c *GimchiClient) messageBinanceFutureOrderBookHandler(data *futures.WsDepthEvent) {
	if data.LastUpdateID < c.BinanceFutureClient.OrderBook.Data.LastUpdateID {

	} else if data.FirstUpdateID <= c.BinanceFutureClient.OrderBook.Data.LastUpdateID+1 && data.LastUpdateID >= c.BinanceFutureClient.OrderBook.Data.LastUpdateID && c.BinanceFutureClient.OrderBook.ProcessID == 0 {
		c.setBinanceFutureOrderBook(data)
	} else if c.BinanceFutureClient.OrderBook.ProcessID == data.LastUpdateID {
		c.setBinanceFutureOrderBook(data)
	} else {
		c.SnapshotBinanceFutureOrderBook()
	}
}

// Binance Future Orderbook 최초 1회 Snapshot 가져오기
func (c *GimchiClient) SnapshotBinanceFutureOrderBook() error {
	orderbook, err := c.BinanceFutureClient.Client.NewDepthService().Symbol(c.BinanceFutureClient.Symbol).Do(c.ctx)

	if err != nil {
		return err
	}

	c.BinanceFutureClient.OrderBook = &model.BinanceFutureOrderBook{}
	c.BinanceFutureClient.OrderBook.ProcessID = 0
	c.BinanceFutureClient.OrderBook.Data = orderbook

	return nil
}

// Binance Future Orderbook WS 에서 받은 걸로 계산 처리
func (c *GimchiClient) setBinanceFutureOrderBook(data *futures.WsDepthEvent) {
	if len(data.Asks) > 0 {
		for _, i := range data.Asks {
			idx, d := c.BinanceFutureClient.OrderBook.BinanceFutureOrderBookFilterAsk(&i)
			if idx == -1 {
				c.BinanceFutureClient.OrderBook.Data.Asks = append(c.BinanceFutureClient.OrderBook.Data.Asks, i)
			} else {
				v, err := strconv.ParseFloat(d.Quantity, 1)
				if err != nil {
					c.Logger.WithError(err).Error("Error convert from string to float")
				}
				if v == 0 {
					c.BinanceFutureClient.OrderBook = c.BinanceFutureClient.OrderBook.BinanceFutureOrderBookRemoveAsk(idx)
				}
			}
		}
	}

	if len(data.Bids) > 0 {
		for _, i := range data.Bids {
			idx, d := c.BinanceFutureClient.OrderBook.BinanceFutureOrderBookFilterBid(&i)
			if idx == -1 {
				c.BinanceFutureClient.OrderBook.Data.Bids = append(c.BinanceFutureClient.OrderBook.Data.Bids, i)
			} else {
				v, err := strconv.ParseFloat(d.Quantity, 1)
				if err != nil {
					c.Logger.WithError(err).Error("Error convert from string to float")
				}
				if v == 0 {
					c.BinanceFutureClient.OrderBook = c.BinanceFutureClient.OrderBook.BinanceFutureOrderBookRemoveBid(idx)
				}
			}
		}
	}

	sort.Slice(c.BinanceFutureClient.OrderBook.Data.Asks, func(i, j int) bool {
		return c.BinanceFutureClient.OrderBook.Data.Asks[i].Price < c.BinanceFutureClient.OrderBook.Data.Asks[j].Price
	})

	sort.Slice(c.BinanceFutureClient.OrderBook.Data.Bids, func(i, j int) bool {
		return c.BinanceFutureClient.OrderBook.Data.Bids[i].Price > c.BinanceFutureClient.OrderBook.Data.Bids[j].Price
	})

	c.BinanceFutureClient.OrderBook.ProcessID = data.LastUpdateID

}

// Binance Future WebSocket User Trade data Handler
func (c *GimchiClient) messageBinanceFutureUserTradeHandler(message []byte) {
	// 주문 전송이 진행중인 경우 끝날때까지 기다린다.
	c.BinanceFutureClient.wg.Wait()

	eventTypeChecker := new(model.EventTypeChecker)
	if err := json.Unmarshal(message, eventTypeChecker); err != nil {
		c.Logger.WithError(err).WithField("msg", string(message)).Error("Failed to check event type.")
		return
	}

	switch eventTypeChecker.EventType {
	case "ORDER_TRADE_UPDATE":
		orderUpdate := new(model.OrderUpdate)
		if err := json.Unmarshal(message, orderUpdate); err != nil {
			c.Logger.WithError(err).WithField("msg", string(message)).Error("Failed to unmarshal order update.")
			return
		}
		c.Logger.WithField("result", orderUpdate.Result).Info()
		if err := c.handleBinanceOrderTradeUpdate(ExchangeTypeBinanceFuture, orderUpdate); err != nil {
			c.Logger.WithField("result", orderUpdate.Result).WithError(err).Error("Failed to handle trade update")
			return
		}
	case "ACCOUNT_UPDATE":
		accountUpdate := new(model.AccountUpdate)
		if err := json.Unmarshal(message, accountUpdate); err != nil {
			c.Logger.WithError(err).WithField("msg", string(message)).Error("Failed to unmarshal account update.")
			return
		}
		c.Logger.WithField("account", accountUpdate.Account).Info()
		if err := c.handleBinanceAccountUpdate(accountUpdate); err != nil {
			c.Logger.WithField("accountUpdate", accountUpdate).WithError(err).Error("Failed to handle account upate")
			return
		}
	case "listenKeyExpired":
		if err := c.RestartBinanceFutureUserStream(); err != nil {
			c.Logger.WithError(err).Error("Failed to restart userstream")
			return
		}
	default:
		c.Logger.WithField("msg", string(message)).Error("Message not handled.")
	}
}

// Binance Future User Trade 의 ORDER_TRADE_UPDATE 데이터를 처리 하는 메소드
func (c *GimchiClient) handleBinanceOrderTradeUpdate(exchangeType ExchangeType, o *model.OrderUpdate) error {
	orderID := o.Result.OrderID

	var err error

	// 각 스텝의 주문정보를 orderID 를 통해 조회한다.
	i, openOrder, err := c.filterOrderWork(exchangeType, orderID)
	if err != nil {
		// orderID 로 못찾을 경우 ClientOrderId 로 다시 한번 찾기
		i, openOrder, err = c.filterOrderWorkByClientOrderId(exchangeType, o.Result.ClientOrderID)
		if err != nil {
			return err
		}
	}

	switch o.Result.OrderStatus {
	case "NEW":
		err = nil
	case "PARTIALLY_FILLED":
		err = nil
	case "FILLED":
		// 주문이 가득 체워진 경우 완료 처리
		openOrder.IsFinished = true
		openOrder.UpdatedAt = common.Now()

		if c.WorkStep == 1 {
			c.WorkInfo.FirstStep.BinanceFutureWorks[i] = openOrder
		} else if c.WorkStep == 2 {
			if exchangeType == ExchangeTypeBinanceFuture {
				c.WorkInfo.SecondStep.BinanceFutureWorks[i] = openOrder
			} else {
				c.WorkInfo.SecondStep.BinanceSpotWorks[i] = openOrder
			}
		}

		fallthrough
	case "CANCELED":
		err = nil
	case "EXPIRED":
		// STOP_MARKET 주문이 체결될 때 기존 주문이 EXPIRED 되고 MARKET 주문이 같은 OrderID로 새로 생성된다.
		err = nil
	case "CALCULATED":
		fallthrough
	case "TRADE":
		fallthrough
	default:
		err = errors.New("Unexpected OrderStatus")
	}

	if err != nil {
		return err
	}

	return nil
}

// Binance Spot User Trade 의 executionReport 데이터를 처리 하는 메소드
func (c *GimchiClient) handleBinanceOrderTradeResult(exchangeType ExchangeType, o *model.SpotOrderResult) error {
	orderID := o.OrderID

	var err error

	// 각 스텝의 주문정보를 orderID 를 통해 조회한다.
	i, openOrder, err := c.filterOrderWork(exchangeType, orderID)
	if err != nil {
		// orderID 로 못찾을 경우 ClientOrderId 로 다시 한번 찾기
		i, openOrder, err = c.filterOrderWorkByClientOrderId(exchangeType, o.ClientOrderID)
		if err != nil {
			return err
		}
	}

	switch o.ExecutionType {
	case "NEW":
		err = nil
	case "CANCELED":
		err = nil
	case "REPLACED":
		err = nil
	case "REJECTED":
		err = nil
	case "TRADE":
		if o.OrderStatus == "FILLED" {
			// 주문이 가득 체워진 경우 완료 처리
			openOrder.IsFinished = true
			openOrder.UpdatedAt = common.Now()

			if c.WorkStep == 1 {
				c.WorkInfo.FirstStep.BinanceFutureWorks[i] = openOrder
			} else if c.WorkStep == 2 {
				if exchangeType == ExchangeTypeBinanceFuture {
					c.WorkInfo.SecondStep.BinanceFutureWorks[i] = openOrder
				} else {
					c.WorkInfo.SecondStep.BinanceSpotWorks[i] = openOrder
				}

				// Second Step 에서 Binance Spot 의 Sell 주문이 완료 시 Binance Future 의 Short Close 를 위한 buy 주문 실행
				err := c.createBinanceFutureOrder(i)
				if err != nil {
					c.Logger.WithError(err).Error("Failed to create order in binance future")
				}

			}
		}
		fallthrough
	case "EXPIRED":
		// STOP_MARKET 주문이 체결될 때 기존 주문이 EXPIRED 되고 MARKET 주문이 같은 OrderID로 새로 생성된다.
		err = nil
	default:
		err = errors.New("Unexpected OrderStatus")
	}

	if err != nil {
		return err
	}

	return nil
}

// 각 스탭 별 실행될 주문 내역을 OrderID 를 통해 조회하는 메소드
func (c *GimchiClient) filterOrderWork(exchangeType ExchangeType, orderId int64) (idx int, data *model.OrderWork, err error) {
	if exchangeType == ExchangeTypeGoPax {
		if c.WorkStep == 1 {
			for i, r := range c.WorkInfo.FirstStep.GoPaxWorks {
				if r.OrderId == orderId {
					return i, r, nil
				}
			}
			return -1, nil, errors.New("not found work by orderId")
		} else {
			return -1, nil, errors.New("gopax only 1 step")
		}
	} else if exchangeType == ExchangeTypeBinanceFuture {
		if c.WorkStep == 1 {
			for i, r := range c.WorkInfo.FirstStep.BinanceFutureWorks {
				if r.OrderId == orderId {
					return i, r, nil
				}
			}
			return -1, nil, errors.New("not found work by orderId")
		} else if c.WorkStep == 2 {
			for i, r := range c.WorkInfo.SecondStep.BinanceFutureWorks {
				if r.OrderId == orderId {
					return i, r, nil
				}
			}
			return -1, nil, errors.New("not found work by orderId")
		} else {
			return -1, nil, errors.New("this step doesn`t have object.")
		}
	} else if exchangeType == ExchangeTypeBinanceSpot {
		if c.WorkStep == 1 {
			return -1, nil, errors.New("this step doesn`t have object.")
		} else if c.WorkStep == 2 {
			for i, r := range c.WorkInfo.SecondStep.BinanceSpotWorks {
				if r.OrderId == orderId {
					return i, r, nil
				}
			}
			return -1, nil, errors.New("not found work by orderId")
		} else {
			return -1, nil, errors.New("this step doesn`t have object.")
		}
	}

	return -1, nil, errors.New("another exchange error")
}

// 각 스탭 별 실행될 주문 내역을 OrderID 를 통해 조회하는 메소드
func (c *GimchiClient) filterOrderWorkByClientOrderId(exchangeType ExchangeType, clientOrderId string) (idx int, data *model.OrderWork, err error) {
	if exchangeType == ExchangeTypeGoPax {
		if c.WorkStep == 1 {
			for i, r := range c.WorkInfo.FirstStep.GoPaxWorks {
				if r.ClientOrderId == clientOrderId {
					return i, r, nil
				}
			}
			return -1, nil, errors.New("not found work by clientOrderId")
		} else {
			return -1, nil, errors.New("gopax only 1 step")
		}
	} else if exchangeType == ExchangeTypeBinanceFuture {
		if c.WorkStep == 1 {
			for i, r := range c.WorkInfo.FirstStep.BinanceFutureWorks {
				if r.ClientOrderId == clientOrderId {
					return i, r, nil
				}
			}
			return -1, nil, errors.New("not found work by clientOrderId")
		} else if c.WorkStep == 2 {
			for i, r := range c.WorkInfo.SecondStep.BinanceFutureWorks {
				if r.ClientOrderId == clientOrderId {
					return i, r, nil
				}
			}
			return -1, nil, errors.New("not found work by clientOrderId")
		} else {
			return -1, nil, errors.New("this step doesn`t have object.")
		}
	} else if exchangeType == ExchangeTypeBinanceSpot {
		if c.WorkStep == 1 {
			return -1, nil, errors.New("this step doesn`t have object.")
		} else if c.WorkStep == 2 {
			for i, r := range c.WorkInfo.SecondStep.BinanceSpotWorks {
				if r.ClientOrderId == clientOrderId {
					return i, r, nil
				}
			}
			return -1, nil, errors.New("not found work by clientOrderId")
		} else {
			return -1, nil, errors.New("this step doesn`t have object.")
		}
	}

	return -1, nil, errors.New("another exchange error")
}

// Position 정보를 처리하는 메소드 ( 사용하지 않고 있음 )
func (c *GimchiClient) handleBinanceAccountUpdate(a *model.AccountUpdate) error {
	// fmt.Println("Account")
	// fmt.Println(a)
	// TODO: Balance 업데이트도 적용해 매번 AvailableBalance를 호출할 필요가 없게 수정한다.

	// 포지션 업데이트
	// for _, p := range a.Account.Positions {
	// 	if p.PositionSide != "BOTH" {
	// 		c.Logger.WithField("position", p).Error("Unexpected position update. Only BOTH is considered.")
	// 		continue
	// 	}
	// 	c.positions[p.Symbol] = p
	// }

	return nil
}

// Gimchi Rate 계산
func (c *GimchiData) getRate() {
	basic := 1 / c.USD * c.KRW

	c.Rate = (c.GopaxPrice/(c.BinancePrice*basic) - 1) * 100
}

// app.json 읽어서 workinfo 가져오기
func (c *GimchiClient) getWorkJson() (data *model.TransferWorkInfo, err error) {
	jsonFile, err := os.Open("app.json")
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var info model.TransferWorkInfo

	err = json.Unmarshal([]byte(byteValue), &info)
	if err != nil {
		return nil, err
	}

	z := &model.ZeroStepWorks{
		CheckRate: info.GimchiPremium,
		FindRate:  0,
		CreatedAt: common.Now(),
		UpdatedAt: 0,
	}

	info.ZeroStep = z

	return &info, nil
}

// Slack Noti 처리
func (c *GimchiClient) SendMessage(title string, msg interface{}) error {
	_msg := Message{
		Title: title,
		Msg:   msg,
	}
	err := c.SlackClient.Client.SendMessage(_msg)
	if err != nil {
		c.Logger.WithError(err).Error("Failed to send message error")
		return err
	}

	// redis publish
	c.RedisPublish(_msg)

	return nil
}

// 김프 모니터링
func (c *GimchiClient) MonitorGimchiPremium() error {
	checker := c.WorkInfo.GimchiPremium

	c.Logger.Info(c.Data)

	// redis publish
	c.RedisPublish(c.Data)

	// 특정값 도달시 다음 스탭 시작 및 stream stop
	if c.Data.Rate != -100 && c.Data.Rate <= checker {

		m := Message{
			Title: "Gimchi Premium Monitoring End!",
			Msg:   "GimchiPremium Check! and Gopax Buy Order and Binance Future Short Start",
		}
		c.RedisPublish(m)

		c.WorkInfo.ZeroStep.UpdatedAt = common.Now()
		c.WorkInfo.ZeroStep.FindRate = c.Data.Rate

		c.SendMessage("GimchiPremium Check! and Gopax Buy Order and Binance Future Short Start", c.Data)
		c.StopBinanceSpotTradeStream()
		c.StopGoPaxTradeStream()

		err := c.GoPaxBuyAndBinanceFutureShort()
		if err != nil {
			c.Logger.WithError(err).Error("GoPaxBuyAndBinanceFutureShort Error")
			return err
		}
	}

	return nil
}

// Redis 에 현재 작업 진행 사항을 Publish 하자.
func (c *GimchiClient) RedisPublish(msg interface{}) {

	byteData, err := json.Marshal(msg)
	if err != nil {
		c.Logger.WithError(err).Error("Failed to publish in redis by marshal")
	}

	err = c.Rds.Publish(c.ctx, "gopax2binance", string(byteData)).Err()
	if err != nil {
		c.Logger.WithError(err).Error("Failed to publish in redis")
	}
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////
// 김프 모니터링 이후 1차 로직 ( gopax 에서 코인 사는 것과 동시에 Binance future 에서 short 잡는 것 )
// 2차 로직 ( Binance Spot 에서 코인 팔고 동시에 Binance future 에서 short 잡힌 것을 푸는 것 )
/////////////////////////////////////////////////////////////////////////////////////////////////////////

// 1차 로직에 필요한 WorkInfo 설정
func (c *GimchiClient) setFirstStep() *GimchiClient {
	count := c.WorkInfo.WorkPrice / c.WorkInfo.ThreadPrice
	mod := c.WorkInfo.WorkPrice % c.WorkInfo.ThreadPrice
	if mod > 0 {
		count = count + 1
	}
	now := common.Now()

	// 1차 로직에는 주문 작업 내역의 기본 정보가 gopax 와 binance future 가 동일하다.
	var list []*model.OrderWork
	for i := 0; i < count; i++ {
		o := &model.OrderWork{
			OrderPrice:  0,
			OrderAmount: 0,
			// OrderFillAmount: 0,
			IsFinished: false,
			IsOrdered:  false,
			CreatedAt:  0,
			UpdatedAt:  0,
		}

		list = append(list, o)
	}
	f := &model.FirstStepWorks{
		WorkLength:         count,
		FinishLength:       0,
		GoPaxWorks:         list,
		BinanceFutureWorks: list,
		CreatedAt:          now,
		UpdatedAt:          0,
		WG:                 &sync.WaitGroup{},
	}
	c.WorkInfo.FirstStep = f
	c.WorkStep = 1

	// binance future order list redis 저장
	var binanceOrderList []int
	binanceOrderList = append(binanceOrderList, -1)
	b, _ := json.Marshal(binanceOrderList)
	_ = c.Rds.Set(c.ctx, "binanceOrderList", b, 0).Err()

	return c
}

// 2차 로직에 필요한 WorkInfo 설정
func (c *GimchiClient) setSecondStep() *GimchiClient {

	now := common.Now()
	count := c.WorkInfo.EqualParts

	// 2차 로직에서는 Balance 를 가지고 Binance Spot 과 Binance Future 의 order 에 필요한 OrderQuantity 를 미리 계산해서 넣어둔다.
	var binanceSpotlist []*model.OrderWork
	for i := 0; i < count; i++ {
		balance, _ := strconv.ParseFloat(c.BinanceBalance.Spot.Free, 8)
		amount := balance / float64(count)

		o := &model.OrderWork{
			OrderPrice:  0,
			OrderAmount: amount,
			// OrderFillAmount: 0,
			IsFinished: false,
			IsOrdered:  false,
			CreatedAt:  0,
			UpdatedAt:  0,
		}

		binanceSpotlist = append(binanceSpotlist, o)
	}

	var binanceFuturelist []*model.OrderWork
	for i := 0; i < count; i++ {
		balance, _ := strconv.ParseFloat(c.BinanceBalance.Future.PositionAmt, 8)
		if balance < 0 {
			balance = math.Abs(balance)
		}
		amount := balance / float64(count)

		o := &model.OrderWork{
			OrderPrice:  0,
			OrderAmount: amount,
			// OrderFillAmount: 0,
			IsFinished: false,
			IsOrdered:  false,
			CreatedAt:  0,
			UpdatedAt:  0,
		}

		binanceFuturelist = append(binanceFuturelist, o)
	}

	f := &model.SecondStepWorks{
		WorkLength:         count,
		FinishLength:       0,
		BinanceSpotWorks:   binanceSpotlist,
		BinanceFutureWorks: binanceFuturelist,
		CreatedAt:          now,
		UpdatedAt:          0,
		WG:                 &sync.WaitGroup{},
	}
	c.WorkInfo.SecondStep = f
	c.WorkStep = 2

	return c
}

// 첫번째 Gopax Buy and Binance Future Short
func (c *GimchiClient) GoPaxBuyAndBinanceFutureShort() error {
	// 첫번째 작업 목록 설정
	c = c.setFirstStep()

	// Gopax Orderbook getter start
	if err := c.StartGoPaxOrderBookStream(); err != nil {
		c.Logger.WithError(err).Error("Failed to start OrderBook by GoPax")
		return err
	}
	// Gopax User Trade getter start
	if err := c.StartGoPaxUserTradesStream(); err != nil {
		c.Logger.WithError(err).Error("Failed to start User Trades by GoPax")
		return err
	}

	// Binance Futures Orderbook getter start
	if err := c.StartBinanceFutureOrderBookStream(); err != nil {
		c.Logger.WithError(err).Error("Failed to start OrderBook by Binance Future")
		return err
	}
	// Binance Future User Trade getter start
	if err := c.StartBinanceFutureUserStream(); err != nil {
		c.Logger.WithError(err).Error("Failed to start User Trades by Binance Future")
		return err
	}

	// First Order Start ( GoPax 부터 주문한다. )
	err := c.goFirstOrderWork()
	if err != nil {
		c.Logger.WithError(err).Error("Failed to start goFirstOrderWork")
		return err
	}

	return nil
}

// 두번째 Binance Spot Sell and Binance Future Short Close
func (c *GimchiClient) BinanceSpotSellAndBinanceFuturePositionClose() error {
	// Binance Spot 과 Future 의 account 정보 조회
	c.BinanceBalance = &BinanceBalance{}
	binanceSpotBalance, err := c.getBinanceSpotAccountInfo()
	if err != nil {
		c.Logger.WithError(err).Error("binance spot no balance")
		return err
	}
	c.BinanceBalance.Spot = binanceSpotBalance

	binanceFutureBalance, err := c.getBinanceFutureAccountInfo()
	if err != nil {
		c.Logger.WithError(err).Error("binance future no balance")
		return err
	}
	c.BinanceBalance.Future = binanceFutureBalance

	// 두번째 작업 목록 설정
	c.setSecondStep()

	// 시장가로 주문하기 때문에 OrderBook 필요 X
	// if err := c.StartBinanceSpotOrderBookStream(); err != nil {
	// 	c.Logger.WithError(err).Error("Failed to start OrderBook by Binance spot")
	// }

	if err := c.StartBinanceSpotUserStream(); err != nil {
		c.Logger.WithError(err).Error("Failed to start user trade by Binance spot")
	}

	// 시장가로 주문하기 때문에 OrderBook 필요 X
	// if err := c.StartBinanceFutureOrderBookStream(); err != nil {
	// 	c.Logger.WithError(err).Error("Failed to start OrderBook by Binance future")
	// }

	if err := c.StartBinanceFutureUserStream(); err != nil {
		c.Logger.WithError(err).Error("Failed to start user trade by Binance future")
	}

	// Second Order Start
	err = c.goSecondOrderWork()
	if err != nil {
		c.Logger.WithError(err).Error("Failed to start goSecondOrderWork")
		return err
	}

	return nil
}

// FirstStep 의 주문 시작 메소드
func (c *GimchiClient) goFirstOrderWork() error {
	time.Sleep(10 * time.Second)

	// 분할해서 매수 시작
	for i := 0; i < c.WorkInfo.FirstStep.WorkLength; i++ {
		err := c.createGoPaxOrder(i)
		if err != nil {

			return err
		}

		time.Sleep(10 * time.Second)
	}

	// 계속해서 Gopax 와 Binance 주문 상태 확인 체크
	for {
		gopaxCount := len(c.WorkInfo.FirstStep.GoPaxWorks)
		binanceCount := len(c.WorkInfo.FirstStep.BinanceFutureWorks)

		gopaxChecker := 0
		binanceChecker := 0

		for i := 0; i < gopaxCount; i++ {
			if c.WorkInfo.FirstStep.GoPaxWorks[i].UpdatedAt != 0 {
				gopaxChecker++
			}
		}

		for i := 0; i < binanceCount; i++ {
			if c.WorkInfo.FirstStep.BinanceFutureWorks[i].UpdatedAt != 0 {
				binanceChecker++
			}
		}

		if gopaxChecker == gopaxCount && binanceChecker == binanceCount {

			c.setWorkInfoFinish()
		}

		if c.WorkInfo.FirstStep.UpdatedAt != 0 {
			m := Message{
				Title: "GoPax2Binance - FirstStep - Finish",
				Msg:   c.WorkInfo.FirstStep,
			}

			c.Logger.Info(m)
			c.SendMessage(m.Title, c.WorkInfo.FirstStep)

			// First step 에 필요한 stream 끄기
			c.StopGoPaxOrderBookStream()
			c.StopGoPaxUserTradesStream()
			c.StopBinanceFutureOrderBookStream()
			c.StopBinanceFutureUserStream()

			break
		}
	}

	return nil
}

// SecondStep 의 주문 시작 메소드
func (c *GimchiClient) goSecondOrderWork() error {
	time.Sleep(10 * time.Second)

	// 분할해서 매수 시작
	for i := 0; i < c.WorkInfo.SecondStep.WorkLength; i++ {
		// err := c.createGoPaxOrder(i)
		err := c.createBinanceSpotOrder(i)
		if err != nil {

			return err
		}

		time.Sleep(10 * time.Second)
	}

	time.Sleep(10 * time.Second)

	// 계속해서 Gopax 와 Binance 주문 상태 확인 체크
	// var isFirstFinish = false
	for {
		spotCount := len(c.WorkInfo.SecondStep.BinanceSpotWorks)
		binanceCount := len(c.WorkInfo.SecondStep.BinanceFutureWorks)

		spotChecker := 0
		binanceChecker := 0

		for i := 0; i < spotCount; i++ {
			if c.WorkInfo.SecondStep.BinanceSpotWorks[i].UpdatedAt != 0 {
				spotChecker++
			}
		}

		for i := 0; i < binanceCount; i++ {
			if c.WorkInfo.SecondStep.BinanceFutureWorks[i].UpdatedAt != 0 {
				binanceChecker++
			}
		}

		if spotChecker == spotCount && binanceChecker == binanceCount {
			// 혹시나 남는 잔고 만큼을 추가로 주문하려고 했는데. 잔고부족 에러가 나서 그냥 주석 처리 함.
			// if !isFirstFinish {
			// 	c.orderBinanceSpotRemainBalance()
			// 	c.orderBinanceFutureRemainBalance()
			// 	isFirstFinish = true
			// } else {
			// 	c.setWorkInfoFinish()
			// }
			c.setWorkInfoFinish()
		}

		if c.WorkInfo.SecondStep.UpdatedAt != 0 {
			m := Message{
				Title: "GoPax2Binance - SecondStep - Finish",
				Msg:   c.WorkInfo.SecondStep,
			}

			c.Logger.Info(m)
			c.SendMessage(m.Title, c.WorkInfo.SecondStep)

			// Second step 에 필요한 stream 끄기
			c.StopBinanceSpotUserStream()
			c.StopBinanceFutureUserStream()

			break
		}
	}

	return nil
}

func (c *GimchiClient) checkGoPaxOrder(orderId int) <-chan bool {

	// c.GoPaxClient.wg.Add(1)

	r := make(chan bool)

	go func() {
		// defer c.GoPaxClient.wg.Done()

		orderResult, err := c.GoPaxClient.Client.NewGetOrderService().OrderId(strconv.Itoa(int(orderId))).Do(c.ctx)
		if err != nil {
			c.Logger.WithError(err).Error("GoPax Order Result API Call Err")
			r <- false
		} else {

			if orderResult.Status == "completed" {
				r <- true
			} else {
				r <- false
			}
		}
	}()

	return r
}

// GoPax 에 orderWork 의 index 에 맞는 주문 내역을 주문하기
func (c *GimchiClient) createGoPaxOrder(i int) error {

	c.GoPaxClient.wg.Add(1)

	resChannel := make(chan *gopax.CreateOrderResponse, 1)
	errChannel := make(chan error, 1)

	go func() {
		defer c.GoPaxClient.wg.Done()

		// GoPax Order
		buyPrice := c.WorkInfo.ThreadPrice
		orderPrice := c.GoPaxClient.OrderBook.Data.Ask[1].Price
		orderAmount := float64(buyPrice) / orderPrice
		// fmt.Println(orderAmount)
		orderAmount = math.Round(orderAmount*1000) / 1000
		// fmt.Println(orderAmount)

		if orderAmount < 0.01 {
			orderAmount = 0.01
		}

		clientOrderId := fmt.Sprintf("gopax_first_%s", strconv.Itoa(i+1))

		c.WorkInfo.FirstStep.GoPaxWorks[i].CreatedAt = common.Now()
		c.WorkInfo.FirstStep.GoPaxWorks[i].OrderPrice = orderPrice
		c.WorkInfo.FirstStep.GoPaxWorks[i].OrderAmount = orderAmount
		c.WorkInfo.FirstStep.GoPaxWorks[i].ClientOrderId = clientOrderId
		c.WorkInfo.FirstStep.GoPaxWorks[i].IsOrdered = true

		res, err := c.GoPaxClient.Client.NewCreateOrderService().ClientOrderID(clientOrderId).TradingPairName(c.GoPaxClient.Symbol).Side("buy").Type("limit").Price(int64(orderPrice)).Amount(orderAmount).Do(c.ctx)
		if err != nil {
			c.Logger.WithError(err).WithField("orderAmount", orderAmount).Error("Failed to gopax order")
			noti := Message{
				Title: "GoPaxOrder Error",
				Msg:   "Failed to gopax order - " + err.Error(),
			}
			c.RedisPublish(noti)
			resChannel <- nil
			errChannel <- err
			return
		}

		// fmt.Println(res)
		// fmt.Println(res.Id)
		orderId, err := strconv.Atoi(res.Id)
		if err != nil {
			c.Logger.WithError(err).Error("Failed to gopax order id not found")
		}
		c.WorkInfo.FirstStep.GoPaxWorks[i].OrderId = int64(orderId)

		m := Message{
			Title: "GoPaxOrder - FirstStep - " + clientOrderId,
			Msg:   res,
		}

		c.Logger.Info(m)
		c.RedisPublish(m)

		// OrderResult 로그 저장은 messageHandler에서 수행
		resChannel <- res
		errChannel <- nil
	}()

	return nil
}

// Binance Spot 에 orderWork 의 index 에 맞는 주문 내역을 주문하기
func (c *GimchiClient) createBinanceSpotOrder(i int) error {
	c.BinanceSpotClient.wg.Add(1)

	data, err := c.getBinanceSpotAccountInfo()
	if err != nil {
		return err
	}
	remainAmount, _ := strconv.ParseFloat(data.Free, 8)
	remainAmount = math.Floor(remainAmount*1000) / 1000

	resChannel := make(chan *binance.CreateOrderResponse, 1)
	errChannel := make(chan error, 1)

	go func() {
		defer c.BinanceSpotClient.wg.Done()

		// Binance Spot Order
		orderAmount := c.WorkInfo.SecondStep.BinanceSpotWorks[i].OrderAmount

		orderAmount = math.Floor(orderAmount*1000) / 1000

		if orderAmount > remainAmount {
			orderAmount = remainAmount
		}

		clientOrderId := fmt.Sprintf("binance_spot_second_%s", strconv.Itoa(i+1))

		c.WorkInfo.SecondStep.BinanceSpotWorks[i].CreatedAt = common.Now()
		c.WorkInfo.SecondStep.BinanceSpotWorks[i].OrderAmount = orderAmount
		c.WorkInfo.SecondStep.BinanceSpotWorks[i].ClientOrderId = clientOrderId

		order := c.BinanceSpotClient.Client.NewCreateOrderService().Symbol(c.BinanceSpotClient.Symbol).Side(binance.SideTypeSell).Type(binance.OrderTypeMarket)
		order.Quantity(strconv.FormatFloat(orderAmount, 'g', -1, 64))
		order.NewClientOrderID(clientOrderId)

		res, err := order.Do(c.ctx)
		if err != nil {
			c.Logger.WithError(err).WithField("orderResponse", orderAmount).WithField("remainAmount", remainAmount).Error("Failed to binance spot order")
			resChannel <- nil
			errChannel <- err
			return
		}

		c.WorkInfo.SecondStep.BinanceSpotWorks[i].OrderId = res.OrderID
		c.WorkInfo.SecondStep.BinanceSpotWorks[i].IsOrdered = true

		m := Message{
			Title: "BinanceSpotOrder - SecondStep - " + clientOrderId,
			Msg:   res,
		}

		c.Logger.Info(m)
		c.RedisPublish(m)

		// OrderResult 로그 저장은 messageHandler에서 수행
		resChannel <- res
		errChannel <- nil
	}()

	return nil
}

func (c *GimchiClient) createBinanceFutureOrderAsync(i int) (data *futures.CreateOrderResponse, err error) {

	// binanceOrderList
	var binanceOrderList []int
	value, err := c.Rds.Get(c.ctx, "binanceOrderList").Result()
	if err != nil {
		c.Logger.WithError(err).Error("Failed to get in redis data")
	}
	err = json.Unmarshal([]byte(value), &binanceOrderList)
	if err != nil {
		c.Logger.WithError(err).Error("Failed to convert interface in redis")
	}

	checker := true
	for _, idx := range binanceOrderList {
		if idx == i {
			checker = false
		}
	}

	if checker {

		// redis 에 저장
		binanceOrderList = append(binanceOrderList, i)
		b, _ := json.Marshal(binanceOrderList)
		_ = c.Rds.Set(c.ctx, "binanceOrderList", b, 0).Err()

		c.BinanceFutureClient.wg.Add(1)
		c.Logger.Info("Binance Future Short Order Start")

		var orderamount float64
		var clientOrderId string
		var order *futures.CreateOrderService

		// Binance Future 는 First Step 과 Second Step 에서 모두 사용 되어 분기로 주문 정보 처리
		if c.WorkStep == 1 {
			// work = c.WorkInfo.FirstStep.BinanceFutureWorks[i]
			// First Step 에서는 Limit Order 로 Sell 처리
			orderamount = c.WorkInfo.FirstStep.GoPaxWorks[i].OrderAmount
			// Binance 는 주문을 소수점 3자리까지 가능
			orderamount = math.Floor(orderamount*1000) / 1000

			c.WorkInfo.FirstStep.BinanceFutureWorks[i].CreatedAt = common.Now()
			c.WorkInfo.FirstStep.BinanceFutureWorks[i].OrderAmount = orderamount
			// c.WorkInfo.FirstStep.BinanceFutureWorks[i].OrderPrice = binanceOrderPrice
			c.WorkInfo.FirstStep.BinanceFutureWorks[i].ClientOrderId = clientOrderId

			clientOrderId = fmt.Sprintf("binance_future_first_%s", strconv.Itoa(i+1))

			// Binance Future Order
			order = c.BinanceFutureClient.Client.NewCreateOrderService().Symbol(c.BinanceFutureClient.Symbol).Side(futures.SideTypeSell).Type(futures.OrderTypeMarket)
			order.Quantity(strconv.FormatFloat(orderamount, 'g', -1, 64))
			order.NewClientOrderID(clientOrderId)

		} else if c.WorkStep == 2 {
			// Second Step 에서는 Market Order 로 Buy 처리
			data, err := c.getBinanceFutureAccountInfo()
			if err != nil {
				return nil, err
			}

			// 남은 position 수량 체크 및 음수로 된 Short 상태를 정수로 주문하기 위해 Abs 로 양수 변환
			remainAmount, _ := strconv.ParseFloat(data.PositionAmt, 8)
			if remainAmount < 0 {
				remainAmount = math.Abs(remainAmount)
			}
			orderamount = c.WorkInfo.SecondStep.BinanceFutureWorks[i].OrderAmount

			if orderamount > remainAmount {
				orderamount = remainAmount
			}
			orderamount = math.Floor(orderamount*1000) / 1000

			c.WorkInfo.SecondStep.BinanceFutureWorks[i].CreatedAt = common.Now()
			c.WorkInfo.SecondStep.BinanceFutureWorks[i].OrderAmount = orderamount
			c.WorkInfo.SecondStep.BinanceFutureWorks[i].ClientOrderId = clientOrderId

			clientOrderId = fmt.Sprintf("binance_future_second_%s", strconv.Itoa(i+1))

			// Binance Future Order
			order = c.BinanceFutureClient.Client.NewCreateOrderService().Symbol(c.BinanceFutureClient.Symbol).Side(futures.SideTypeBuy).Type(futures.OrderTypeMarket)
			order.Quantity(strconv.FormatFloat(orderamount, 'g', -1, 64))
			order.NewClientOrderID(clientOrderId)
		}

		resChannel := make(chan *futures.CreateOrderResponse, 1)
		errChannel := make(chan error, 1)

		go func() {

			defer c.BinanceFutureClient.wg.Done()

			res, err := order.Do(c.ctx)
			if err != nil {
				c.Logger.WithError(err).WithField("orderAmount", orderamount).Error("Failed to create order in binance future.")
				resChannel <- nil
				errChannel <- err
				return
			}

			title := ""
			if c.WorkStep == 1 {
				c.WorkInfo.FirstStep.BinanceFutureWorks[i].OrderId = res.OrderID
				c.WorkInfo.FirstStep.BinanceFutureWorks[i].IsOrdered = true
				title = "Binance Future - FirstStep - " + clientOrderId
			} else if c.WorkStep == 2 {
				c.WorkInfo.SecondStep.BinanceFutureWorks[i].OrderId = res.OrderID
				c.WorkInfo.SecondStep.BinanceFutureWorks[i].IsOrdered = true
				title = "Binance Future - SecondStep - " + clientOrderId
			}

			m := Message{
				Title: title,
				Msg:   res,
			}

			c.Logger.Info(m)
			c.RedisPublish(m)

			// OrderResult 로그 저장은 messageHandler에서 수행
			resChannel <- res
			errChannel <- nil
		}()

		data = <-resChannel
		err = <-errChannel

		return data, err

	} else {
		c.Logger.Info("Skip Binance Future Order by " + strconv.Itoa(i))
		return nil, nil
	}
}

// Binance Future 에 orderWork 의 index 에 맞는 주문 내역을 주문하기
func (c *GimchiClient) createBinanceFutureOrder(i int) error {

	c.BinanceFutureClient.wg.Add(1)

	c.Logger.Info("Binance Future Short Order Start")

	var orderamount float64
	var clientOrderId string
	var order *futures.CreateOrderService

	// Binance Future 는 First Step 과 Second Step 에서 모두 사용 되어 분기로 주문 정보 처리
	if c.WorkStep == 1 {
		// work = c.WorkInfo.FirstStep.BinanceFutureWorks[i]
		// First Step 에서는 Limit Order 로 Sell 처리
		orderamount = c.WorkInfo.FirstStep.GoPaxWorks[i].OrderAmount
		// Binance 는 주문을 소수점 3자리까지 가능
		orderamount = math.Floor(orderamount*1000) / 1000

		// 1차 로직도 market 주문 날리기 때문에 orderbook 볼 필요 없음.
		// binanceOrderPrice, err := strconv.ParseFloat(c.BinanceFutureClient.OrderBook.Data.Bids[1].Price, 2)
		// if err != nil {
		// 	c.Logger.WithError(err).Error("Failed convert binance future orderbook price")
		// 	return err
		// }

		c.WorkInfo.FirstStep.BinanceFutureWorks[i].CreatedAt = common.Now()
		c.WorkInfo.FirstStep.BinanceFutureWorks[i].OrderAmount = orderamount
		// c.WorkInfo.FirstStep.BinanceFutureWorks[i].OrderPrice = binanceOrderPrice
		c.WorkInfo.FirstStep.BinanceFutureWorks[i].ClientOrderId = clientOrderId

		clientOrderId = fmt.Sprintf("binance_future_first_%s", strconv.Itoa(i+1))

		// Binance Future Order
		// order = c.BinanceFutureClient.Client.NewCreateOrderService().Symbol(c.BinanceFutureClient.Symbol).Side(futures.SideTypeSell).Type(futures.OrderTypeLimit).TimeInForce(futures.TimeInForceTypeGTC)
		// order.Quantity(strconv.FormatFloat(orderamount, 'g', -1, 64))
		// order.Price(c.BinanceFutureClient.OrderBook.Data.Bids[1].Price)
		// order.NewClientOrderID(clientOrderId)
		order = c.BinanceFutureClient.Client.NewCreateOrderService().Symbol(c.BinanceFutureClient.Symbol).Side(futures.SideTypeSell).Type(futures.OrderTypeMarket)
		order.Quantity(strconv.FormatFloat(orderamount, 'g', -1, 64))
		order.NewClientOrderID(clientOrderId)

	} else if c.WorkStep == 2 {
		// work = c.WorkInfo.SecondStep.BinanceFutureWorks[i]
		// Second Step 에서는 Market Order 로 Buy 처리
		data, err := c.getBinanceFutureAccountInfo()
		if err != nil {
			return err
		}

		// 남은 position 수량 체크 및 음수로 된 Short 상태를 정수로 주문하기 위해 Abs 로 양수 변환
		remainAmount, _ := strconv.ParseFloat(data.PositionAmt, 8)
		if remainAmount < 0 {
			remainAmount = math.Abs(remainAmount)
		}
		orderamount = c.WorkInfo.SecondStep.BinanceFutureWorks[i].OrderAmount

		if orderamount > remainAmount {
			orderamount = remainAmount
		}
		orderamount = math.Floor(orderamount*1000) / 1000

		c.WorkInfo.SecondStep.BinanceFutureWorks[i].CreatedAt = common.Now()
		c.WorkInfo.SecondStep.BinanceFutureWorks[i].OrderAmount = orderamount
		c.WorkInfo.SecondStep.BinanceFutureWorks[i].ClientOrderId = clientOrderId

		clientOrderId = fmt.Sprintf("binance_future_second_%s", strconv.Itoa(i+1))

		// Binance Future Order
		order = c.BinanceFutureClient.Client.NewCreateOrderService().Symbol(c.BinanceFutureClient.Symbol).Side(futures.SideTypeBuy).Type(futures.OrderTypeMarket)
		order.Quantity(strconv.FormatFloat(orderamount, 'g', -1, 64))
		order.NewClientOrderID(clientOrderId)
	}

	var isOrder bool
	isOrder = true

	if c.WorkStep == 1 {
		isOrder = c.WorkInfo.FirstStep.BinanceFutureWorks[i].IsOrdered
	} else if c.WorkStep == 2 {
		isOrder = c.WorkInfo.SecondStep.BinanceFutureWorks[i].IsOrdered
	}

	fmt.Println(isOrder)
	// 중복이 아닌 경우 주문 실행
	if isOrder {
		resChannel := make(chan *futures.CreateOrderResponse, 1)
		errChannel := make(chan error, 1)

		go func() {

			defer c.BinanceFutureClient.wg.Done()

			res, err := order.Do(c.ctx)
			if err != nil {
				c.Logger.WithError(err).WithField("orderAmount", orderamount).Error("Failed to create order in binance future.")
				resChannel <- nil
				errChannel <- err
				return
			}

			title := ""
			if c.WorkStep == 1 {
				c.WorkInfo.FirstStep.BinanceFutureWorks[i].OrderId = res.OrderID
				c.WorkInfo.FirstStep.BinanceFutureWorks[i].IsOrdered = true
				title = "Binance Future - FirstStep - " + clientOrderId
			} else if c.WorkStep == 2 {
				c.WorkInfo.SecondStep.BinanceFutureWorks[i].OrderId = res.OrderID
				c.WorkInfo.SecondStep.BinanceFutureWorks[i].IsOrdered = true
				title = "Binance Future - SecondStep - " + clientOrderId
			}

			m := Message{
				Title: title,
				Msg:   res,
			}

			c.Logger.Info(m)
			c.RedisPublish(m)

			// OrderResult 로그 저장은 messageHandler에서 수행
			resChannel <- res
			errChannel <- nil
		}()
	}

	return nil
}

// 단계별 workInfo 완료 처리
func (c *GimchiClient) setWorkInfoFinish() error {
	if c.WorkStep == 1 {
		c.WorkInfo.FirstStep.UpdatedAt = common.Now()
	} else {
		c.WorkInfo.SecondStep.UpdatedAt = common.Now()
	}

	return nil
}

func (c *GimchiClient) getGopaxAccountInfo() (data []*gopax.Balance, err error) {
	res, err := c.GoPaxClient.Client.NewGetBalancesService().Do(c.ctx)
	if err != nil {
		c.Logger.WithError(err).Error("Gopax Balance check")
		return nil, err
	}

	return res, nil
}

// Binance Spot 의 Balance 정보 가져오는 메소드
func (c *GimchiClient) getBinanceSpotAccountInfo() (data *binance.Balance, err error) {
	res, err := c.BinanceSpotClient.Client.NewGetAccountService().Do(c.ctx)
	if err != nil {
		c.Logger.WithError(err).Error("Binance Spot Balance check")
		return nil, err
	}

	var _data *binance.Balance
	for _, r := range res.Balances {
		if r.Asset == c.WorkInfo.WorkSymbol {
			_data = &r
			break
		}
	}

	if _data == nil {
		return nil, errors.New("no have balance of work symbol in binance spot")
	}

	data = _data

	return data, nil
}

// Binance Future 의 Position 정보 가져오는 메소드
func (c *GimchiClient) getBinanceFutureAccountInfo() (data *futures.AccountPosition, err error) {
	res, err := c.BinanceFutureClient.Client.NewGetAccountService().Do(c.ctx)
	if err != nil {
		return nil, err
	}

	var _data *futures.AccountPosition
	for _, r := range res.Positions {
		if r.Symbol == c.BinanceFutureClient.Symbol {
			_data = r
			break
		}
	}

	if _data == nil {
		return nil, errors.New("no have position of work symbol in binance future")
	}

	data = _data

	return data, nil
}

// func (c *GimchiClient) orderBinanceSpotRemainBalance() error {
// 	data, err := c.getBinanceSpotAccountInfo()
// 	if err != nil {
// 		return err
// 	}

// 	amount, err := strconv.ParseFloat(data.Free, 8)
// 	if err != nil {
// 		return err
// 	}

// 	if amount > 0 {
// 		c.BinanceSpotClient.wg.Add(1)

// 		resChannel := make(chan *binance.CreateOrderResponse, 1)
// 		errChannel := make(chan error, 1)

// 		o := model.OrderWork{
// 			OrderId:     0,
// 			OrderPrice:  0,
// 			OrderAmount: amount,
// 			IsFinished:  false,
// 			CreatedAt:   common.Now(),
// 			UpdatedAt:   0,
// 		}
// 		c.WorkInfo.SecondStep.BinanceSpotWorks = append(c.WorkInfo.SecondStep.BinanceSpotWorks, &o)

// 		go func() {
// 			defer c.BinanceSpotClient.wg.Done()

// 			// Binance Spot Order
// 			i := len(c.WorkInfo.SecondStep.BinanceSpotWorks) - 1

// 			clientOrderId := fmt.Sprintf("binance_spot_second_%s", strconv.Itoa(i))
// 			order := c.BinanceSpotClient.Client.NewCreateOrderService().Symbol(c.BinanceSpotClient.Symbol).Side(binance.SideTypeSell).Type(binance.OrderTypeMarket)
// 			order.Quantity(strconv.FormatFloat(amount, 'g', 1, 64))
// 			order.NewClientOrderID(clientOrderId)

// 			res, err := order.Do(c.ctx)
// 			if err != nil {
// 				c.Logger.WithError(err).Error("Failed to binance spot order")
// 				resChannel <- nil
// 				errChannel <- err
// 				return
// 			}

// 			c.WorkInfo.SecondStep.BinanceSpotWorks[i].OrderId = res.OrderID

// 			m := Message{
// 				Title: "BinanceSpotOrder - SecondStep - " + clientOrderId,
// 				Msg:   res,
// 			}

// 			c.Logger.Info(m)
// 			c.RedisPublish(m)

// 			// OrderResult 로그 저장은 messageHandler에서 수행
// 			resChannel <- res
// 			errChannel <- nil
// 		}()
// 	}

// 	return nil
// }

// func (c *GimchiClient) orderBinanceFutureRemainBalance() error {
// 	data, err := c.getBinanceFutureAccountInfo()
// 	if err != nil {
// 		return err
// 	}

// 	amount, err := strconv.ParseFloat(data.PositionAmt, 8)
// 	if err != nil {
// 		return err
// 	}

// 	if amount > 0 {
// 		c.BinanceFutureClient.wg.Add(1)

// 		resChannel := make(chan *futures.CreateOrderResponse, 1)
// 		errChannel := make(chan error, 1)

// 		o := model.OrderWork{
// 			OrderId:     0,
// 			OrderPrice:  0,
// 			OrderAmount: amount,
// 			IsFinished:  false,
// 			CreatedAt:   common.Now(),
// 			UpdatedAt:   0,
// 		}
// 		c.WorkInfo.SecondStep.BinanceFutureWorks = append(c.WorkInfo.SecondStep.BinanceFutureWorks, &o)

// 		go func() {
// 			defer c.BinanceFutureClient.wg.Done()

// 			// Binance Spot Order
// 			i := len(c.WorkInfo.SecondStep.BinanceFutureWorks) - 1

// 			clientOrderId := fmt.Sprintf("binance_future_second_%s", strconv.Itoa(i))
// 			order := c.BinanceFutureClient.Client.NewCreateOrderService().Symbol(c.BinanceFutureClient.Symbol).Side(futures.SideTypeBuy).Type(futures.OrderTypeMarket).TimeInForce(futures.TimeInForceTypeGTC)
// 			order.Quantity(strconv.FormatFloat(amount, 'g', 1, 64))
// 			order.NewClientOrderID(clientOrderId)

// 			res, err := order.Do(c.ctx)
// 			if err != nil {
// 				c.Logger.WithError(err).Error("Failed to binance future order")
// 				resChannel <- nil
// 				errChannel <- err
// 				return
// 			}

// 			c.WorkInfo.SecondStep.BinanceFutureWorks[i].OrderId = res.OrderID

// 			m := Message{
// 				Title: "BinanceFutureOrder - SecondStep - " + clientOrderId,
// 				Msg:   res,
// 			}

// 			c.Logger.Info(m)
// 			c.RedisPublish(m)

// 			// OrderResult 로그 저장은 messageHandler에서 수행
// 			resChannel <- res
// 			errChannel <- nil
// 		}()
// 	}

// 	return nil
// }
