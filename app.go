package main

import (
	"net/http"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	rice "github.com/GeertJohan/go.rice"
	"github.com/foolin/echo-template/supports/gorice"

	"arques.com/transfer/api"
	"arques.com/transfer/worker"
)

func main() {
	// go worker.ConnectionGoPax()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// static resource 사용
	staticBox := rice.MustFindBox("static")
	staticFileServer := http.StripPrefix("/static/", http.FileServer(staticBox.HTTPBox()))
	e.GET("/static/*", echo.WrapHandler(staticFileServer))

	// echo-template
	e.Renderer = gorice.New(rice.MustFindBox("views"))

	e.GET("/", api.MainPage)

	e.GET("/transferlog", api.TransferLog)
	e.GET("/workinfo", worker.GetWorkInfo)
	e.GET("/gimchi", api.GimchiPremium)
	e.POST("/setworkinfo", api.SetWorkInfo)
	e.GET("/checkbalancegopax", api.CheckGopaxBalance)
	e.GET("/checkbalance", api.CheckBinanceSpotBalance)
	e.GET("/checkbalancefuture", api.CheckBinanceFutureBalance)

	e.GET("/firststart", api.StartTransferFirstStep)
	e.GET("/secondstart", api.StartTransferSecondStep)

	e.GET("/tradebinancefuture", api.GetTradeBinanceFuture)

	e.Logger.Fatal(e.Start(":80"))
}
