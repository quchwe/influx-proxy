// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	gzip "github.com/klauspost/pgzip"
)

var (
	ErrBadRequest   = errors.New("bad request")
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
	ErrInternal     = errors.New("internal error")
	ErrUnknown      = errors.New("unknown error")
)

type QueryResult struct {
	Header http.Header
	Status int
	Body   []byte
	Err    error
}

type HttpBackend struct { // nolint:golint
	client    *http.Client
	transport *http.Transport
	URL       string
	DB        string
	Username  string
	Password  string
	interval  int
	active    atomic.Value
	rewriting atomic.Value
	writeOnly bool
}

// TODO: query timeout? use req.Cancel
func NewHttpBackend(cfg *BackendConfig) (hb *HttpBackend) { // nolint:golint
	hb = &HttpBackend{
		client: &http.Client{
			Transport: NewTransport(strings.HasPrefix(cfg.URL, "https")),
			Timeout:   time.Millisecond * time.Duration(cfg.Timeout),
		},
		transport: NewTransport(strings.HasPrefix(cfg.URL, "https")),
		URL:       cfg.URL,
		DB:        cfg.DB,
		Username:  cfg.Username,
		Password:  cfg.Password,
		interval:  cfg.CheckInterval,
		writeOnly: cfg.WriteOnly,
	}
	hb.active.Store(true)
	hb.rewriting.Store(false)
	go hb.CheckActive()
	return
}

func NewTransport(tlsSkip bool) (transport *http.Transport) {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   time.Second * 30,
			KeepAlive: time.Second * 30,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       time.Second * 90,
		TLSHandshakeTimeout:   time.Second * 10,
		ExpectContinueTimeout: time.Second * 1,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: tlsSkip},
	}
}

func CloneQueryRequest(r *http.Request) *http.Request {
	cr := r.Clone(r.Context())
	cr.Body = ioutil.NopCloser(&bytes.Buffer{})
	return cr
}

func Compress(buf *bytes.Buffer, p []byte) (err error) {
	zip := gzip.NewWriter(buf)
	n, err := zip.Write(p)
	if err != nil {
		return
	}
	if n != len(p) {
		err = io.ErrShortWrite
		return
	}
	err = zip.Close()
	return
}

func CopyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Set(k, v)
		}
	}
}

// TODO: update active when calling successed or failed.
func (hb *HttpBackend) CheckActive() {
	var err error
	for {
		_, err = hb.Ping()
		hb.active.Store(err == nil)
		time.Sleep(time.Millisecond * time.Duration(hb.interval))
	}
}

func (hb *HttpBackend) IsActive() (b bool) {
	return hb.active.Load().(bool)
}

func (hb *HttpBackend) IsRewriting() (b bool) {
	return hb.rewriting.Load().(bool)
}

func (hb *HttpBackend) SetRewriting(b bool) {
	hb.rewriting.Store(b)
}

func (hb *HttpBackend) IsWriteOnly() (b bool) {
	return hb.writeOnly
}

func (hb *HttpBackend) Ping() (version string, err error) {
	resp, err := hb.client.Get(hb.URL + "/ping")
	if err != nil {
		log.Print("http error: ", err)
		return
	}
	defer resp.Body.Close()

	version = resp.Header.Get("X-Influxdb-Version")

	if resp.StatusCode == 204 {
		return
	}
	log.Printf("ping status code: %d, the backend is %s", resp.StatusCode, hb.URL)

	respbuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("readall error: ", err)
		return
	}
	log.Printf("error response: %s", respbuf)
	return
}

func (hb *HttpBackend) QuerySink(req *http.Request) (qr *QueryResult) {
	qr = &QueryResult{}
	if len(req.Form) == 0 {
		req.Form = url.Values{}
	}
	req.Form.Set("db", hb.DB)
	req.ContentLength = 0
	if hb.Username != "" || hb.Password != "" {
		req.Form.Set("u", hb.Username)
		req.Form.Set("p", hb.Password)
	}

	req.URL, qr.Err = url.Parse(hb.URL + "/query?" + req.Form.Encode())
	if qr.Err != nil {
		log.Print("internal url parse error: ", qr.Err)
		return
	}

	q := strings.TrimSpace(req.FormValue("q"))
	resp, err := hb.transport.RoundTrip(req)
	if err != nil {
		if req.Header.Get("Query-Origin") != "Parallel" || err.Error() != "context canceled" {
			qr.Err = err
			log.Printf("query error: %s, the query is %s", err, q)
		}
		return
	}
	defer resp.Body.Close()

	respBody := resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		b, err := gzip.NewReader(resp.Body)
		if err != nil {
			qr.Err = err
			log.Printf("unable to decode gzip body: %s", err)
			return
		}
		defer b.Close()
		respBody = b
	}

	qr.Body, qr.Err = ioutil.ReadAll(respBody)
	if qr.Err != nil {
		log.Printf("read body error: %s, the query is %s", qr.Err, q)
		return
	}

	if resp.StatusCode >= 400 {
		rsp, _ := ResponseFromResponseBytes(qr.Body)
		qr.Err = errors.New(rsp.Err)
	}
	qr.Header = resp.Header
	qr.Status = resp.StatusCode
	return
}

// Don't setup Accept-Encoding: gzip. Let real client do so.
// If real client don't support gzip and we setted, it will be a mistake.
func (hb *HttpBackend) Query(w http.ResponseWriter, req *http.Request) (err error) {
	if len(req.Form) == 0 {
		req.Form = url.Values{}
	}
	req.Form.Set("db", hb.DB)
	req.ContentLength = 0
	if hb.Username != "" || hb.Password != "" {
		req.Form.Set("u", hb.Username)
		req.Form.Set("p", hb.Password)
	}

	req.URL, err = url.Parse(hb.URL + "/query?" + req.Form.Encode())
	if err != nil {
		log.Print("internal url parse error: ", err)
		return
	}

	q := strings.TrimSpace(req.FormValue("q"))
	resp, err := hb.transport.RoundTrip(req)
	if err != nil {
		log.Printf("query error: %s, the query is %s", err, q)
		return
	}
	defer resp.Body.Close()

	CopyHeader(w.Header(), resp.Header)

	p, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("read body error: %s, the query is %s", err, q)
		return
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(p)
	return
}

func (hb *HttpBackend) Write(p []byte) (err error) {
	var buf bytes.Buffer
	err = Compress(&buf, p)
	if err != nil {
		log.Print("compress error: ", err)
		return
	}

	log.Printf("http backend write %s", hb.DB)
	err = hb.WriteStream(&buf, true)
	return
}

func (hb *HttpBackend) WriteCompressed(p []byte) (err error) {
	buf := bytes.NewBuffer(p)
	err = hb.WriteStream(buf, true)
	return
}

func (hb *HttpBackend) WriteStream(stream io.Reader, compressed bool) (err error) {
	q := url.Values{}
	q.Set("db", hb.DB)
	if hb.Username != "" || hb.Password != "" {
		q.Set("u", hb.Username)
		q.Set("p", hb.Password)
	}

	req, err := http.NewRequest("POST", hb.URL+"/write?"+q.Encode(), stream)
	if compressed {
		req.Header.Add("Content-Encoding", "gzip")
	}

	resp, err := hb.client.Do(req)
	if err != nil {
		log.Print("http error: ", err)
		hb.active.Store(false)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		return
	}
	log.Printf("write status code: %d, the backend is %s", resp.StatusCode, hb.URL)

	respbuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("readall error: ", err)
		return
	}
	log.Printf("error response: %s", respbuf)

	// translate code to error
	// https://docs.influxdata.com/influxdb/v1.1/tools/api/#write
	switch resp.StatusCode {
	case 400:
		err = ErrBadRequest
	case 401:
		err = ErrUnauthorized
	case 404:
		err = ErrNotFound
	case 500:
		err = ErrInternal
	default: // mostly tcp connection timeout, or request entity too large
		err = ErrUnknown
	}
	return
}

func (hb *HttpBackend) Close() (err error) {
	hb.transport.CloseIdleConnections()
	return
}
