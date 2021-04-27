package gopax

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	base64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"arques.com/transfer/common"
	"github.com/bitly/go-simplejson"
)

type (
	Client struct {
		APIKey     string
		SecretKey  string
		BaseURL    string
		UserAgent  string
		HTTPClient *http.Client
		Debug      bool
		Logger     *log.Logger
		TimeOffset int64
		do         doFunc
	}

	doFunc func(req *http.Request) (*http.Response, error)
)

const (
	timestampKey  = "timestamp"
	signatureKey  = "signature"
	recvWindowKey = "recvWindow"
)

func currentTimestamp() int64 {
	return int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)
}

func newJSON(data []byte) (j *simplejson.Json, err error) {
	j, err = simplejson.NewJson(data)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func NewClient(apiKey, secretKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		SecretKey:  secretKey,
		BaseURL:    "https://api.gopax.co.kr",
		UserAgent:  "Gopax/golang",
		HTTPClient: http.DefaultClient,
		Logger:     log.New(os.Stderr, "Gopax-golang", log.LstdFlags),
	}
}

func (c *Client) debug(format string, v ...interface{}) {
	if c.Debug {
		c.Logger.Printf(format, v...)
	}
}

func (c *Client) parseRequest(r *request, opts ...RequestOption) (err error) {
	// set request options from user
	for _, opt := range opts {
		opt(r)
	}
	err = r.validate()
	if err != nil {
		return err
	}

	fullURL := fmt.Sprintf("%s%s", c.BaseURL, r.endpoint)
	if r.recvWindow > 0 {
		r.setParam(recvWindowKey, r.recvWindow)
	}
	if r.secType == secTypeSigned {
		r.setParam(timestampKey, currentTimestamp()-c.TimeOffset)
	}
	// queryString := r.query.Encode()
	body := &bytes.Buffer{}
	bodyString := r.form.Encode()
	header := http.Header{}

	var bodyBytes []byte = nil
	if r.param != nil {
		bodyBytes, _ = json.Marshal(r.param)
	}

	if bodyString != "" {
		header.Set("Content-Type", "application/x-www-form-urlencoded")
		body = bytes.NewBufferString(bodyString)
	}

	if r.secType == secTypeAPIKey || r.secType == secTypeSigned {
		// header.Set("X-MBX-APIKEY", c.APIKey)
		timestamp := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
		msg := "t" + timestamp + r.method
		if r.method == "GET" && strings.HasPrefix(r.endpoint, "/orders?") {
			msg += r.endpoint
		} else {
			msg += strings.Split(r.endpoint, "?")[0]
		}
		if r.recvWindow != -1 {
			msg += strconv.Itoa(int(r.recvWindow))
		}

		if bodyBytes != nil {
			msg += string(bodyBytes)
		}

		rawSecret, _ := base64.StdEncoding.DecodeString(c.SecretKey)
		mac := hmac.New(sha512.New, rawSecret)
		mac.Write([]byte(msg))
		rawSignature := mac.Sum(nil)
		signature := base64.StdEncoding.EncodeToString(rawSignature)

		header.Set("api-key", c.APIKey)
		header.Set("timestamp", timestamp)
		header.Set("signature", signature)
		if r.recvWindow != -1 {
			header.Set("receive-window", strconv.Itoa(int(r.recvWindow)))
		}

	}

	c.debug("full url: %s, body: %s", fullURL, bodyString)

	// fmt.Println(fullURL)
	// fmt.Println(header)
	// fmt.Println(bodyString)
	// fmt.Println(string(bodyBytes))

	r.fullURL = fullURL
	r.header = header
	r.body = body
	r.bodyByte = bodyBytes
	return nil
}

func (c *Client) callAPI(ctx context.Context, r *request, opts ...RequestOption) (data []byte, err error) {
	err = c.parseRequest(r, opts...)
	if err != nil {
		return []byte{}, err
	}

	// req, err := http.NewRequest(r.method, r.fullURL, r.body)
	req, err := http.NewRequest(r.method, r.fullURL, bytes.NewBuffer(r.bodyByte))
	if err != nil {
		return []byte{}, err
	}
	req = req.WithContext(ctx)
	req.Header = r.header
	c.debug("request: %#v", req)
	f := c.do
	if f == nil {
		f = c.HTTPClient.Do
	}
	res, err := f(req)
	if err != nil {
		return []byte{}, err
	}

	if res.StatusCode != 200 {
		code := strconv.Itoa(res.StatusCode)
		fmt.Println("gopax api http status code = " + code)
	}

	data, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return []byte{}, err
	}
	defer func() {
		cerr := res.Body.Close()
		// Only overwrite the retured error if the original error was nil and an
		// error occurred while closing the body.
		if err == nil && cerr != nil {
			err = cerr
		}
	}()

	c.debug("response: %#v", res)
	c.debug("response body: %s", string(data))
	c.debug("response status code: %d", res.StatusCode)

	// fmt.Println(strconv.Itoa(res.StatusCode))

	if res.StatusCode >= 400 {
		apiErr := new(common.APIError)
		e := json.Unmarshal(data, apiErr)
		if e != nil {
			c.debug("failed to unmarshal json: %s", e)
		}
		return nil, apiErr
	}
	return data, nil
}

func (c *Client) NewGetBalancesService() *GetBalancesService {
	return &GetBalancesService{c: c}
}

func (c *Client) NewGetBalanceService() *GetBalanceService {
	return &GetBalanceService{c: c}
}

func (c *Client) NewGetTradeService() *GetTradesService {
	return &GetTradesService{c: c}
}

func (c *Client) NewGetOrdersService() *GetOrdersService {
	return &GetOrdersService{c: c}
}

func (c *Client) NewGetOrderService() *GetOrderService {
	return &GetOrderService{c: c}
}

func (c *Client) NewCreateOrderService() *CreateOrderService {
	return &CreateOrderService{c: c}
}

func (c *Client) NewDeleteOrderService() *DeleteOrderService {
	return &DeleteOrderService{c: c}
}

func (c *Client) NewGetDepositWithdrawalStatusService() *GetDepositWithdrawalStatusService {
	return &GetDepositWithdrawalStatusService{c: c}
}

func (c *Client) NewGetCryptoDepositAddressService() *GetCryptoDepositAddressService {
	return &GetCryptoDepositAddressService{c: c}
}

func (c *Client) NewGetCryptoWithdrawalAddressService() *GetCryptoWithdrawalAddressService {
	return &GetCryptoWithdrawalAddressService{c: c}
}

func (c *Client) NewGetTickerService() *GetTickerService {
	return &GetTickerService{c: c}
}
