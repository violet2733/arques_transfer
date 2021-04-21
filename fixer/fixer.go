package fixer

import (
	"context"
	"encoding/json"
	"fmt"

	"arques.com/transfer/common"
	"arques.com/transfer/config"
	"arques.com/transfer/model"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type (
	Client struct {
		APIKey  string
		BaseURL string
		Rds     *redis.Client
		ctx     context.Context
		Logger  *log.Entry
	}
)

func NewClient(baseURL, apiKey string) *Client {
	c := &Client{}

	c.APIKey = apiKey
	c.BaseURL = baseURL
	c.ctx = context.Background()

	// 클라이언트 로거 설정
	c.Logger = log.WithFields(log.Fields{
		"workType": "gimchi",
	})

	// 레디스 클라이언트 생성
	c.Rds = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Get().Redis.Url, config.Get().Redis.Port),
		Password: "",
		DB:       1,
	})

	return c
}

func (c *Client) setApiUrl() string {
	url := fmt.Sprintf("%s?access_key=%s", c.BaseURL, c.APIKey)
	return url
}

func (c *Client) GetExchangeRateData() (data *model.MoneyExchangeRate, err error) {
	now := common.Now()

	// c.Logger.Info("get data start!")
	rData, err := c.GetRedisData()
	if err != nil {
		return nil, err
	}

	rDataCheckTime := rData.Timestamp * 1000

	compare := common.CompareTimestamps(rDataCheckTime, now)

	if compare > 12 {
		data, err = c.GetLiveData()
		if err != nil {
			return rData, nil
		} else {
			err := c.SetRedisData(data)
			if err != nil {
				c.Logger.WithError(err).Error("Error set redis data of live data and get redis data")
				return rData, nil
			}
			// c.Logger.Info("get live data")
		}
	} else {
		data = rData
		// c.Logger.Info("get redis data")
	}

	return data, nil
}

func (c *Client) GetRedisData() (data *model.MoneyExchangeRate, err error) {
	key := "gopax2binance_fixer_data"
	// c.Logger.Info(key)
	value, err := c.Rds.Get(c.ctx, key).Bytes()

	if err != nil {
		c.Logger.WithError(err).Error("Error get data")

		data, err = c.GetLiveData()
		if err != nil {
			c.Logger.WithError(err).Error("Error no redis and live data")
			return nil, err
		} else {
			// c.Logger.Info("no redis data and get live data")
			err = c.SetRedisData(data)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}

	err = json.Unmarshal(value, &data)
	if err != nil {
		c.Logger.WithError(err).Error("Error json unmarshal")
		return nil, err
	}
	// c.Logger.Info("get redis data")

	return data, nil
}

func (c *Client) SetRedisData(data *model.MoneyExchangeRate) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	key := "gopax2binance_fixer_data"
	err = c.Rds.Set(c.ctx, key, b, 0).Err()

	if err != nil {
		c.Logger.WithError(err).Error("Error set redis data!")
		return err
	}

	c.Logger.Info("set redis data")
	return nil
}

func (c *Client) GetLiveData() (data *model.MoneyExchangeRate, err error) {
	url := c.setApiUrl()

	common.GetJson(url, &data)
	return data, nil
}
