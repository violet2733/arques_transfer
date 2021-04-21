package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type (
	Client struct {
		BaseURL string
		Channel string
		Message interface{}
	}
)

func NewClient(baseUrl, channelName string) *Client {
	c := &Client{}
	c.BaseURL = baseUrl
	c.Channel = channelName

	return c
}

func (c *Client) SendMessage(msg interface{}) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	param := fmt.Sprintf("?channel=%s&message=%s", url.QueryEscape(c.Channel), url.QueryEscape(string(b)))

	res, err := http.Get(c.BaseURL + param)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
