package httpclient

import (
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

//var dNSPool = []string{"1.1.1.1", "1.0.0.1", "9.9.9.9", "8.8.8.8", "8.8.4.4", "208.67.222.222", "208.67.222.222"}

type redirects struct {
	Count       int
	Urls        []string
	StatusCodes []int
	Sizes       []int
}

type middleware struct {
	transport   http.Transport
	redirects   *redirects
	analyzer    bool
	maxRetry    int
	readTimeout time.Duration
	maxBodySize int64
}

func (m middleware) appendRedirect(url string, statuscode int, size int) {
	m.redirects.Count += 1
	m.redirects.Urls = append(m.redirects.Urls, url)
	m.redirects.StatusCodes = append(m.redirects.StatusCodes, statuscode)
	m.redirects.Sizes = append(m.redirects.Sizes, size)
}

func (m middleware) analyze(r *Response) {

	//TODO detect some security bugs

	return
}

func (m middleware) RoundTrip(req *http.Request) (resp *http.Response, err error) {

	transport := m.transport
	//proxyUrl, _ := url.Parse("http://127.0.0.1:8080")
	//transport.Proxy = http.ProxyURL(proxyUrl)
	transport.MaxIdleConns = 2000
	transport.MaxIdleConnsPerHost = 1000
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true, Renegotiation: tls.RenegotiateOnceAsClient}
	transport.DisableKeepAlives = true
	transport.DialContext = (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 1 * time.Second,
	}).DialContext

	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.IdleConnTimeout = 15 * time.Second
	transport.ExpectContinueTimeout = 5 * time.Second
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.DisableCompression = true
	var retried = 0
	for {
		resp, err = transport.RoundTrip(req)
		if err != nil {
			if retried == m.maxRetry {
				return
			}
			retried += 1
			continue
		} else {
			break
		}
	}

	//prevent redirect to other domain...
	if req.Response != nil && req.Response.Request != nil {
		if !match(*req.URL, *req.Response.Request.URL) {
			resp.Body.Close()
			return nil, nil
		}
	}

	if resp.StatusCode > 299 && resp.StatusCode < 400 {
		var body []byte
		readDone := make(chan int)
		go func() {
			body, _ = ioutil.ReadAll(io.LimitReader(resp.Body, m.maxBodySize))
			readDone <- 1
		}()

		select {
		case <-time.After(m.readTimeout):
			//deadline
		case <-readDone:
		}
		resp.Body.Close()
		size := len(body)
		m.appendRedirect(req.URL.String(), resp.StatusCode, size)
		resp.Body.Close()
	}

	return
}
