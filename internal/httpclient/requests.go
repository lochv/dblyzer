package httpclient

import (
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

const userAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.120 Safari/537.36"
const defaultMaxBodySize = int64(1 << 18)
const defaultReadTimeout = 20 * time.Second

type Response struct {
	StatusCode int
	Status     string
	Proto      string
	Redirects  redirects
	Size       int
	Text       string
	Headers    http.Header
	Cookies    []*http.Cookie
	Success    bool
	Path       string
	CommonName string
}

type Client struct {
	middleware       *middleware
	Session          bool
	Following        bool
	httpClient       *http.Client
	cookies          http.CookieJar
	DisableUrlEncode bool
	Analyze          bool
	Retry            int
	ReadTimeout      time.Duration
	MaxBodySize      int64
}

func (c *Client) preReq() {

	if c.ReadTimeout == 0 {
		c.ReadTimeout = defaultReadTimeout
	}
	if c.MaxBodySize == 0 {
		c.MaxBodySize = defaultMaxBodySize
	}

	c.middleware = &middleware{
		transport:   http.Transport{},
		redirects:   nil,
		analyzer:    false,
		maxRetry:    c.Retry,
		readTimeout: c.ReadTimeout,
		maxBodySize: c.MaxBodySize,
	}

	c.middleware.redirects = &redirects{Count: 0, Urls: nil, StatusCodes: nil}
	if c.Session {
		if c.cookies == nil {
			c.cookies = &jar{perURL: make(map[string][]*http.Cookie)}
		}
	}
	c.httpClient = &http.Client{Transport: c.middleware, Jar: c.cookies}

	if !c.Following {
		c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
}

func (c *Client) req(req *http.Request) Response {
	r := Response{Success: false}
	resp, err := c.httpClient.Do(req)

	if err != nil || resp == nil {
		return r
	}

	r.StatusCode = resp.StatusCode
	r.Redirects = *c.middleware.redirects
	r.Success = true
	r.Headers = resp.Header
	r.Cookies = resp.Cookies()
	r.Path = resp.Request.URL.Path
	r.Status = resp.Status
	r.Proto = resp.Proto
	if resp.TLS != nil {
		r.CommonName = resp.TLS.PeerCertificates[0].Subject.CommonName
	}
	var body []byte
	var readDone = make(chan int)

	go func() {
		body, _ = ioutil.ReadAll(io.LimitReader(resp.Body, c.MaxBodySize))
		readDone <- 1
	}()

	select {
	case <-time.After(c.ReadTimeout):
		//deadline
	case <-readDone:
	}
	r.Text = string(body)
	r.Size = len(body)
	resp.Body.Close()
	c.middleware.analyze(&r)
	return r
}
