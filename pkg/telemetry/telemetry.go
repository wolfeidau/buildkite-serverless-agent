package telemetry

import (
	"time"

	"github.com/buildkite/agent/api"
	"github.com/sirupsen/logrus"
)

type measurement struct {
	Action   string        `json:"action"`
	Duration time.Duration `json:"duration"`
}

type response struct {
	Host          string `json:"host"`
	Method        string `json:"method"`
	RequestURI    string `json:"uri"`
	StatusCode    int    `json:"code"`
	ContentLength int64  `json:"len"`
}

// MeasureSince log a duration metric for a given action
func MeasureSince(action string, now time.Time) {
	logrus.WithField("measurement", &measurement{
		Action:   action,
		Duration: time.Since(now),
	}).Println("telemetry")
}

// ReportAPIResponse provide a summary of a http response
func ReportAPIResponse(res *api.Response) {
	var path string
	if res.Request.URL != nil {
		path = res.Request.URL.Path
	}
	logrus.WithField("res", &response{
		Host:          res.Request.Host,
		RequestURI:    path,
		Method:        res.Request.Method,
		StatusCode:    res.StatusCode,
		ContentLength: res.ContentLength,
	}).Println("telemetry")
}
