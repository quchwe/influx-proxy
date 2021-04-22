// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package monitor

import (
	"time"

	client "github.com/influxdata/influxdb1-client/v2"
)

type Metric struct {
	Name   string                 `json:"name"`
	Tags   map[string]string      `json:"tags"`
	Fields map[string]interface{} `json:"fields"`
	Time   time.Time              `json:"time"`
}

func (m *Metric) ParseToLine() (line string, err error) {
	p, err := client.NewPoint(m.Name, m.Tags, m.Fields, m.Time)
	if err != nil {
		return "", err
	}
	line = p.PrecisionString("ns")

	return
}
