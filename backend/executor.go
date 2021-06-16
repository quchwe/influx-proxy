// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/influxdata/influxdb1-client/models"
)

type InfluxQLExecutor struct {
	ic *InfluxCluster
}

func GzipCompress(b []byte) (cb []byte, err error) {
	var buf bytes.Buffer
	zip := gzip.NewWriter(&buf)
	n, err := zip.Write(b)
	if err != nil {
		return
	}
	if n != len(b) {
		err = io.ErrShortWrite
		return
	}
	err = zip.Close()
	cb = buf.Bytes()
	return
}

func Write(w http.ResponseWriter, body []byte, gzip bool) {
	if gzip {
		gzipBody, err := GzipCompress(body)
		if err != nil {
			log.Printf("gzip compress error: %s", err)
			rsp := ResponseFromError(err.Error())
			body = rsp.Marshal(false)
			w.Write(body)
			return
		}
		w.Write(gzipBody)
	} else {
		w.Write(body)
	}
}

func WriteResp(w http.ResponseWriter, req *http.Request, rsp *Response, header http.Header) {
	CopyHeader(w.Header(), header)
	w.Header().Del("Content-Length")
	if rsp == nil {
		rsp = ResponseFromSeries(nil)
	}
	pretty := req.FormValue("pretty") == "true"
	body := rsp.Marshal(pretty)
	Write(w, body, header.Get("Content-Encoding") == "gzip")
}

func (iqe *InfluxQLExecutor) Query(w http.ResponseWriter, req *http.Request, tokens []string) (err error) {
	stmt := strings.ToLower(tokens[0])
	if stmt == "show" {
		return iqe.QueryShowQL(w, req, tokens)
	} else if stmt == "create" {
		return iqe.QueryCreateQL(w, req, tokens)
	} else if stmt == "delete" || stmt == "drop" {
		return iqe.QueryDeleteOrDropQL(w, req, tokens)
	}
	return ErrIllegalQL
}

func (iqe *InfluxQLExecutor) QueryShowQL(w http.ResponseWriter, req *http.Request, tokens []string) (err error) {
	// remove support of query parameter `chunked`
	req.Form.Del("chunked")
	backends := make([]BackendAPI, 0)
	for _, api := range iqe.ic.backends {
		backends = append(backends, api)
	}
	bodies, header, inactive, err := QueryInParallel(backends, req, nil)
	if err != nil {
		return err
	}
	var rsp *Response
	if len(bodies) == 0 {
		rsp = ResponseFromSeries(nil)
	} else {
		stmt2 := GetHeadStmtFromTokens(tokens, 2)
		stmt3 := GetHeadStmtFromTokens(tokens, 3)
		if stmt2 == "show measurements" || stmt2 == "show series" || stmt2 == "show databases" {
			rsp, err = reduceByValues(bodies)
		} else if stmt3 == "show field keys" || stmt3 == "show tag keys" || stmt3 == "show tag values" {
			rsp, err = reduceBySeries(bodies)
		} else if stmt3 == "show retention policies" {
			rsp, err = concatByValues(bodies)
		} else if stmt2 == "show stats" {
			rsp, err = concatByResults(bodies)
		} else {
			rsp = ResponseFromSeries(nil)
		}
	}
	if err != nil {
		return
	}
	if inactive > 0 {
		rsp.Err = fmt.Sprintf("%d/%d backends unavailable", inactive, len(backends))
	}
	WriteResp(w, req, rsp, header)
	return
}

func (iqe *InfluxQLExecutor) QueryCreateQL(w http.ResponseWriter, req *http.Request, tokens []string) (err error) {
	if GetHeadStmtFromTokens(tokens, 2) == "create database" {
		backends := make([]BackendAPI, 0)
		for _, api := range iqe.ic.backends {
			backends = append(backends, api)
		}
		fn := func(api BackendAPI, cr *http.Request) {
			hb := api.(*Backends)
			cr.Form.Del("db")
			if len(tokens) >= 3 {
				tokens2 := make([]string, len(tokens))
				copy(tokens2, tokens)
				tokens2[2] = hb.DB
				cr.Form.Set("q", strings.Join(tokens2, " "))
			} else {
				cr.Form.Set("q", "create database "+hb.DB)
			}
		}
		_, header, inactive, err := QueryInParallel(backends, req, fn)
		if err != nil {
			return err
		}
		rsp := ResponseFromSeries(nil)
		if inactive > 0 {
			rsp.Err = fmt.Sprintf("%d/%d backends unavailable", inactive, len(backends))
		}
		WriteResp(w, req, rsp, header)
		return nil
	}
	return ErrIllegalQL
}

func (iqe *InfluxQLExecutor) QueryDeleteOrDropQL(w http.ResponseWriter, req *http.Request, tokens []string) (err error) {
	if CheckDeleteOrDropMeasurementFromTokens(tokens) {
		key, err := GetMeasurementFromTokens(tokens)
		if err != nil {
			return ErrGetMeasurement
		}
		apis, ok := iqe.ic.GetBackends(key)
		if !ok {
			return ErrUnknownMeasurement
		}
		_, header, inactive, err := QueryInParallel(apis, req, nil)
		if err != nil {
			return err
		}
		rsp := ResponseFromSeries(nil)
		if inactive > 0 {
			rsp.Err = fmt.Sprintf("%d/%d backends unavailable", inactive, len(apis))
		}
		WriteResp(w, req, rsp, header)
		return nil
	}
	return ErrIllegalQL
}

func QueryInParallel(backends []BackendAPI, req *http.Request, fn func(BackendAPI, *http.Request)) (bodies [][]byte, header http.Header, inactive int, err error) {
	var wg sync.WaitGroup
	req.Header.Set("Query-Origin", "Parallel")
	ch := make(chan *QueryResult, len(backends))
	for _, api := range backends {
		if !api.IsActive() {
			inactive++
			continue
		}
		wg.Add(1)
		go func(api BackendAPI) {
			defer wg.Done()
			cr := CloneQueryRequest(req)
			if fn != nil {
				fn(api, cr)
			}
			ch <- api.QuerySink(cr)
		}(api)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	for qr := range ch {
		if qr.Err != nil {
			err = qr.Err
			return
		}
		header = qr.Header
		bodies = append(bodies, qr.Body)
	}
	return
}

func reduceByValues(bodies [][]byte) (rsp *Response, err error) {
	var series models.Rows
	var values [][]interface{}
	valuesMap := make(map[string][]interface{})
	for _, b := range bodies {
		_series, err := SeriesFromResponseBytes(b)
		if err != nil {
			return nil, err
		}
		if len(_series) == 1 {
			series = _series
			for _, value := range _series[0].Values {
				key := value[0].(string)
				if !strings.HasPrefix(key, StatisticsMeasurementName) {
					valuesMap[key] = value
				}
			}
		}
	}
	if len(series) == 1 {
		for _, value := range valuesMap {
			values = append(values, value)
		}
		if len(values) > 0 {
			series[0].Values = values
		} else {
			series = nil
		}
	}
	return ResponseFromSeries(series), nil
}

func reduceBySeries(bodies [][]byte) (rsp *Response, err error) {
	var series models.Rows
	seriesMap := make(map[string]*models.Row)
	for _, b := range bodies {
		_series, err := SeriesFromResponseBytes(b)
		if err != nil {
			return nil, err
		}
		for _, serie := range _series {
			if serie.Name != StatisticsMeasurementName {
				seriesMap[serie.Name] = serie
			}
		}
	}
	for _, serie := range seriesMap {
		series = append(series, serie)
	}
	return ResponseFromSeries(series), nil
}

func concatByValues(bodies [][]byte) (rsp *Response, err error) {
	var series models.Rows
	var values [][]interface{}
	for _, b := range bodies {
		_series, err := SeriesFromResponseBytes(b)
		if err != nil {
			return nil, err
		}
		if len(_series) == 1 {
			series = _series
			values = append(values, _series[0].Values...)
		}
	}
	if len(series) == 1 {
		series[0].Values = values
	}
	return ResponseFromSeries(series), nil
}

func concatByResults(bodies [][]byte) (rsp *Response, err error) {
	var results []*Result
	for _, b := range bodies {
		_results, err := ResultsFromResponseBytes(b)
		if err != nil {
			return nil, err
		}
		if len(_results) == 1 {
			results = append(results, _results[0])
		}
	}
	return ResponseFromResults(results), nil
}
