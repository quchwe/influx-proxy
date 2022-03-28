// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import (
	"bytes"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chengshiwen/influx-proxy/util"
)

type Proxy struct {
	Circles []*Circle
}

func NewProxy(cfg *ProxyConfig) (ip *Proxy) {
	err := util.MakeDir(cfg.DataDir)
	if err != nil {
		log.Fatalf("create data dir error: %s", err)
		return
	}
	ip = &Proxy{
		Circles: make([]*Circle, len(cfg.Circles)),
	}
	for idx, circfg := range cfg.Circles {
		ip.Circles[idx] = NewCircle(circfg, cfg, idx)
	}
	rand.Seed(time.Now().UnixNano())
	return
}

func GetKey(org, bucket, meas string) string {
	var b strings.Builder
	b.Grow(len(org) + len(bucket) + len(meas) + 1)
	b.WriteString(org)
	b.WriteString(",")
	b.WriteString(bucket)
	b.WriteString(",")
	b.WriteString(meas)
	return b.String()
}

func (ip *Proxy) GetBackends(key string) []*Backend {
	backends := make([]*Backend, len(ip.Circles))
	for i, circle := range ip.Circles {
		backends[i] = circle.GetBackend(key)
	}
	return backends
}

func (ip *Proxy) GetAllBackends() []*Backend {
	capacity := 0
	for _, circle := range ip.Circles {
		capacity += len(circle.Backends)
	}
	backends := make([]*Backend, 0, capacity)
	for _, circle := range ip.Circles {
		backends = append(backends, circle.Backends...)
	}
	return backends
}

func (ip *Proxy) GetHealth() []interface{} {
	var wg sync.WaitGroup
	health := make([]interface{}, len(ip.Circles))
	for i, c := range ip.Circles {
		wg.Add(1)
		go func(i int, c *Circle) {
			defer wg.Done()
			health[i] = c.GetHealth()
		}(i, c)
	}
	wg.Wait()
	return health
}

func (ip *Proxy) Query(w http.ResponseWriter, req *http.Request, org, query string) (err error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return ErrEmptyQuery
	}
	bucket, meas, err := ScanQuery(query)
	if err != nil {
		return
	}
	if bucket != "" && meas != "" {
		return QueryWithBucketMeasurement(w, req, ip, org, bucket, meas)
	} else if bucket == "" {
		return ErrGetBucket
	} else if meas == "" {
		return ErrGetMeasurement
	}
	return ErrIllegalFluxQuery
}

func (ip *Proxy) Write(p []byte, org, bucket, precision string) (err error) {
	buf := bytes.NewBuffer(p)
	var line []byte
	for {
		line, err = buf.ReadBytes('\n')
		switch err {
		default:
			log.Printf("error: %s", err)
			return
		case io.EOF, nil:
			err = nil
		}
		if len(line) == 0 {
			break
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		ip.WriteRow(line, org, bucket, precision)
	}
	return
}

func (ip *Proxy) WriteRow(line []byte, org, bucket, precision string) {
	nanoLine := AppendNano(line, precision)
	meas, err := ScanKey(nanoLine)
	if err != nil {
		log.Printf("scan key error: %s", err)
		return
	}
	if !RapidCheck(nanoLine[len(meas):]) {
		log.Printf("invalid format, drop data: %s %s %s %s", org, bucket, precision, string(line))
		return
	}

	key := GetKey(org, bucket, meas)
	backends := ip.GetBackends(key)
	if len(backends) == 0 {
		log.Printf("write data error: can't get backends, org: %s, bucket: %s, meas: %s", org, bucket, meas)
		return
	}

	point := &LinePoint{org, bucket, nanoLine}
	for _, be := range backends {
		err = be.WritePoint(point)
		if err != nil {
			log.Printf("write data to buffer error: %s, %s, %s, %s, %s, %s", err, be.Url, org, bucket, precision, string(line))
		}
	}
}

func (ip *Proxy) Close() {
	for _, c := range ip.Circles {
		c.Close()
	}
}
