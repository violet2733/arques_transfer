package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"arques.com/transfer/model"
	"arques.com/transfer/worker"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo"
)

var (
	upgrader = websocket.Upgrader{}
)

func MainPage(c echo.Context) error {
	return c.Render(http.StatusOK, "work/main", echo.Map{})
}

func StartTransferFirstStep(c echo.Context) error {
	go worker.StartGoPax2BinanceFirstStep()

	return c.JSON(http.StatusOK, "ok")
}

func StartTransferSecondStep(c echo.Context) error {
	go worker.StartGoPax2BinanceSecondStep()

	return c.JSON(http.StatusOK, "ok")
}

func TransferLog(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	for {
		s := worker.ConnectionWsTransfer(ws)
		s.Subscribe()

		// // Write
		// err := ws.WriteMessage(websocket.TextMessage, []byte("Hello, Client!"))
		// if err != nil {
		// 	c.Logger().Error(err)
		// }

		// // Read
		// _, msg, err := ws.ReadMessage()
		// if err != nil {
		// 	c.Logger().Error(err)
		// }
		// fmt.Printf("%s\n", msg)
	}
}

func GimchiPremium(c echo.Context) error {
	data, err := worker.GimchiCalculator()

	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.JSON(http.StatusOK, data)
}

func SetWorkInfo(c echo.Context) error {

	reqWork := new(model.TransferWorkInfo)

	if err := c.Bind(reqWork); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	b, err := json.Marshal(reqWork)
	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	// work := c.FormValue("work")
	// b := []byte(work)

	workfile := "app.json"
	err = ioutil.WriteFile(workfile, b, 0644)
	if err != nil {
		return c.JSON(http.StatusOK, "no")
	}

	return c.JSON(http.StatusOK, "ok")
}

func CheckBinanceSpotBalance(c echo.Context) error {
	data, err := worker.CheckBinanceSpotBalance()

	if err != nil {
		return c.JSON(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, data)
}

func CheckBinanceFutureBalance(c echo.Context) error {
	data, err := worker.CheckBinanceFutureBalance()

	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.JSON(http.StatusOK, data)
}
