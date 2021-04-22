// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/chengshiwen/influx-proxy/monitor"
)

var (
	ErrEmptyBackends       = errors.New("backends cannot be empty")
	ErrEmptyKeyMaps        = errors.New("keymaps cannot be empty")
	ErrBackendNotExist     = errors.New("backend not exists")
	ErrMethodNotAllowed    = errors.New("method not allowed")
	ErrEmptyQuery          = errors.New("empty query")
	ErrOnClauseForbidden   = errors.New("on clause forbidden")
	ErrDatabaseNotFound    = errors.New("database not found")
	ErrDatabaseForbidden   = errors.New("database forbidden")
	ErrQueryError          = errors.New("query error")
	ErrGetMeasurement      = errors.New("can't get measurement")
	ErrUnknownMeasurement  = errors.New("unknown measurement")
	ErrBackendsUnavailable = errors.New("backends unavailable")
)

var StatisticsMeasurementName = "influx.proxy.statistics"

type InfluxCluster struct {
	query_executor *InfluxQLExecutor // nolint:golint
	cfgsrc         *FileConfigSource
	backends       map[string]BackendAPI
	m2bs           map[string][]BackendAPI // measurements to backends
	stats          *Statistics
	counter        *Statistics
	ticker         *time.Ticker
	defaultTags    map[string]string
	datadir        string
	DB             string
	WriteTracing   bool
	QueryTracing   bool
}

type Statistics struct {
	QueryRequests        int64
	QueryRequestsFail    int64
	WriteRequests        int64
	WriteRequestsFail    int64
	PingRequests         int64
	PingRequestsFail     int64
	PointsWritten        int64
	PointsWrittenFail    int64
	WriteRequestDuration int64
	QueryRequestDuration int64
}

func NewInfluxCluster(cfgsrc *FileConfigSource, nodecfg *NodeConfig) (ic *InfluxCluster) {
	ic = &InfluxCluster{
		query_executor: &InfluxQLExecutor{},
		cfgsrc:         cfgsrc,
		stats:          &Statistics{},
		counter:        &Statistics{},
		ticker:         time.NewTicker(time.Millisecond * time.Duration(nodecfg.StatInterval)),
		defaultTags:    map[string]string{"addr": nodecfg.ListenAddr},
		datadir:        nodecfg.DataDir,
		DB:             nodecfg.DB,
		WriteTracing:   nodecfg.WriteTracing,
		QueryTracing:   nodecfg.QueryTracing,
	}
	ic.query_executor.ic = ic
	host, err := os.Hostname()
	if err != nil {
		log.Println(err)
	}
	ic.defaultTags["host"] = host

	// feature
	go ic.statistics()
	return
}

func (ic *InfluxCluster) statistics() {
	// how to quit
	for {
		<-ic.ticker.C
		ic.Flush()
		ic.counter = (*Statistics)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&ic.stats)),
			unsafe.Pointer(ic.counter)))
		err := ic.WriteStatistics()
		if err != nil {
			log.Println(err)
		}
	}
}

func (ic *InfluxCluster) Flush() {
	ic.counter.QueryRequests = 0
	ic.counter.QueryRequestsFail = 0
	ic.counter.WriteRequests = 0
	ic.counter.WriteRequestsFail = 0
	ic.counter.PingRequests = 0
	ic.counter.PingRequestsFail = 0
	ic.counter.PointsWritten = 0
	ic.counter.PointsWrittenFail = 0
	ic.counter.WriteRequestDuration = 0
	ic.counter.QueryRequestDuration = 0
}

func (ic *InfluxCluster) WriteStatistics() (err error) {
	metric := &monitor.Metric{
		Name: StatisticsMeasurementName,
		Tags: ic.defaultTags,
		Fields: map[string]interface{}{
			"statQueryRequest":         ic.counter.QueryRequests,
			"statQueryRequestFail":     ic.counter.QueryRequestsFail,
			"statWriteRequest":         ic.counter.WriteRequests,
			"statWriteRequestFail":     ic.counter.WriteRequestsFail,
			"statPingRequest":          ic.counter.PingRequests,
			"statPingRequestFail":      ic.counter.PingRequestsFail,
			"statPointsWritten":        ic.counter.PointsWritten,
			"statPointsWrittenFail":    ic.counter.PointsWrittenFail,
			"statQueryRequestDuration": ic.counter.QueryRequestDuration,
			"statWriteRequestDuration": ic.counter.WriteRequestDuration,
		},
		Time: time.Now(),
	}
	line, err := metric.ParseToLine()
	if err != nil {
		return
	}
	return ic.Write([]byte(line+"\n"), "ns")
}

func (ic *InfluxCluster) loadBackends() (backends map[string]BackendAPI, err error) {
	backends = make(map[string]BackendAPI)

	bkcfgs, err := ic.cfgsrc.LoadBackends()
	if err != nil {
		return
	}

	if len(bkcfgs) == 0 {
		err = ErrEmptyBackends
		return
	}

	for name, cfg := range bkcfgs {
		backends[name], err = NewBackends(cfg, name, ic.datadir)
		if err != nil {
			log.Printf("create backend error: %s", err)
			return
		}
	}

	return
}

func (ic *InfluxCluster) loadMeasurements(backends map[string]BackendAPI) (m2bs map[string][]BackendAPI, err error) {
	m2bs = make(map[string][]BackendAPI)

	m_map, err := ic.cfgsrc.LoadMeasurements() // nolint:golint
	if err != nil {
		return
	}

	if len(m_map) == 0 {
		err = ErrEmptyKeyMaps
		return
	}

	// nolint:golint
	for name, bs_names := range m_map {
		var bss []BackendAPI
		for _, bs_name := range bs_names {
			bs, ok := backends[bs_name]
			if !ok {
				err = ErrBackendNotExist
				log.Println(bs_name, err)
				continue
			}
			bss = append(bss, bs)
		}
		m2bs[name] = bss
	}
	return
}

func (ic *InfluxCluster) LoadConfig() (err error) {
	backends, err := ic.loadBackends()
	if err != nil {
		return
	}

	m2bs, err := ic.loadMeasurements(backends)
	if err != nil {
		return
	}

	ic.backends = backends
	ic.m2bs = m2bs
	return
}

func (ic *InfluxCluster) Ping() (version string, err error) {
	atomic.AddInt64(&ic.stats.PingRequests, 1)
	version = Version
	return
}

func (ic *InfluxCluster) GetBackends(key string) (backends []BackendAPI, ok bool) {
	backends, ok = ic.m2bs[key]
	// match use prefix
	if !ok {
		for k, v := range ic.m2bs {
			if strings.HasPrefix(key, k) {
				backends = v
				ok = true
				break
			}
		}
	}

	if !ok {
		backends, ok = ic.m2bs["_default_"]
	}

	return
}

func (ic *InfluxCluster) Query(w http.ResponseWriter, req *http.Request) (err error) {
	atomic.AddInt64(&ic.stats.QueryRequests, 1)
	defer func(start time.Time) {
		atomic.AddInt64(&ic.stats.QueryRequestDuration, time.Since(start).Nanoseconds())
	}(time.Now())

	switch req.Method {
	case "GET", "POST":
	default:
		atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
		return ErrMethodNotAllowed
	}

	// TODO: multi queries in q?
	q := strings.TrimSpace(req.FormValue("q"))
	if q == "" {
		atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
		return ErrEmptyQuery
	}

	tokens, check, from := CheckQuery(q)
	if !check {
		atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
		return ErrIllegalQL
	}

	checkDb, showDb, db := CheckDatabaseFromTokens(tokens)
	if !checkDb {
		if _, err := GetDatabaseFromTokens(tokens); err == nil {
			// on clause forbidden
			return ErrOnClauseForbidden
		}
		db = req.FormValue("db")
	}
	if !showDb {
		if db == "" {
			atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
			return ErrDatabaseNotFound
		}
		if ic.DB != "" && db != ic.DB {
			atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
			return ErrDatabaseForbidden
		}
	}

	if !from || !CheckSelectOrShowFromTokens(tokens) {
		err = ic.query_executor.Query(w, req, tokens)
		if err != nil {
			atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
		}
		return
	}

	key, err := GetMeasurementFromTokens(tokens)
	if err != nil {
		atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
		return ErrGetMeasurement
	}

	apis, ok := ic.GetBackends(key)
	if !ok {
		log.Printf("unknown measurement: %s", key)
		atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
		return ErrUnknownMeasurement
	}

	// pass non-active, rewriting or write-only.
	for _, api := range apis {
		if !api.IsActive() || api.IsRewriting() || api.IsWriteOnly() {
			continue
		}
		err = api.Query(w, req)
		if err == nil {
			return
		}
	}

	// pass non-active, non-writing (excluding rewriting and write-only).
	for _, api := range apis {
		if !api.IsActive() || !(api.IsRewriting() || api.IsWriteOnly()) {
			continue
		}
		err = api.Query(w, req)
		if err == nil {
			return
		}
	}

	if err == nil {
		backends := make([]string, len(apis))
		for i, api := range apis {
			hb := api.(*Backends)
			backends[i] = hb.URL
		}
		log.Printf("backends unavailable: %v", backends)
		atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
		return ErrBackendsUnavailable
	}
	atomic.AddInt64(&ic.stats.QueryRequestsFail, 1)
	return ErrQueryError
}

// Wrong in one row will not stop others.
// So don't try to return error, just print it.
func (ic *InfluxCluster) WriteRow(line []byte) {
	atomic.AddInt64(&ic.stats.PointsWritten, 1)

	// empty line, ignore it.
	if len(line) == 0 {
		return
	}

	key, err := ScanKey(line)
	if err != nil {
		log.Printf("scan key error: %s", err)
		atomic.AddInt64(&ic.stats.PointsWrittenFail, 1)
		return
	}

	if !RapidCheck(line[len(key):]) {
		log.Printf("invalid format, drop data: %s", string(line))
		atomic.AddInt64(&ic.stats.PointsWrittenFail, 1)
		return
	}

	bs, ok := ic.GetBackends(key)
	if !ok {
		log.Printf("new measurement: %s", key)
		atomic.AddInt64(&ic.stats.PointsWrittenFail, 1)
		return
	}

	// don't block here for a long time, we just have one worker.
	for _, b := range bs {
		err = b.Write(line)
		if err != nil {
			log.Printf("cluster write fail: %s", key)
			atomic.AddInt64(&ic.stats.PointsWrittenFail, 1)
			return
		}
	}
}

func (ic *InfluxCluster) Write(p []byte, precision string) (err error) {
	atomic.AddInt64(&ic.stats.WriteRequests, 1)
	defer func(start time.Time) {
		atomic.AddInt64(&ic.stats.WriteRequestDuration, time.Since(start).Nanoseconds())
	}(time.Now())

	buf := bytes.NewBuffer(p)

	var line []byte
	for {
		line, err = buf.ReadBytes('\n')
		switch err {
		default:
			log.Printf("error: %s", err)
			atomic.AddInt64(&ic.stats.WriteRequestsFail, 1)
			return
		case io.EOF, nil:
			err = nil
		}

		if len(line) == 0 {
			break
		}

		line = AppendNano(line, precision)
		ic.WriteRow(line)
	}

	return
}

func (ic *InfluxCluster) Close() (err error) {
	for name, bs := range ic.backends {
		err = bs.Close()
		if err != nil {
			log.Printf("fail in close backend %s", name)
		}
	}
	return
}
