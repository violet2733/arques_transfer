package common

import (
	"encoding/json"
	"net/http"
	"time"
)

func GetJson(url string, data interface{}) error {
	var client = &http.Client{Timeout: 10 * time.Second}
	r, err := client.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(data)
}

func Now() int64 {
	return time.Now().UTC().UnixNano() / int64(time.Millisecond)
}

func ConvertTimeByTimestamp(timestamp int64) time.Time {
	timestamp = timestamp * int64(time.Millisecond)
	timeT := time.Unix(0, timestamp)
	return timeT
}

func CompareTimestamps(prev, next int64) float64 {
	_prev := ConvertTimeByTimestamp(prev)
	_next := ConvertTimeByTimestamp(next)

	diff := _next.Sub(_prev)

	return diff.Hours()
}
