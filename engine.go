package dblyzer

import (
	"crypto/md5"
	"dblyzer/internal/dbio"
	"dblyzer/internal/httpclient"
	"fmt"
	"strings"
)

type engine struct {
	in     chan dbio.Receive
	out    chan dbio.Report
	w      *Wappalyzer
	header map[string]string
}

func newEngine(filePath string, in chan dbio.Receive, out chan dbio.Report) *engine {
	e := &engine{
		in:     in,
		out:    out,
		w:      newWappalyzer(filePath, 40),
		header: nil,
	}
	return e
}

func (this *engine) scan(rc dbio.Receive) dbio.Report {

	var rp dbio.Report

	rp.Host = rc.Host
	rp.Ip = rc.Ip
	rp.Port = rc.Port

	if isHTTPS(rc.Service) {
		rc.Service = "https"
	} else if isHTTP(rc.Service) {
		rc.Service = "http"
	}

	rp.Service = rc.Service

	if rc.Service != "http" && rc.Service != "https" {
		return rp
	}

	client := httpclient.Client{
		Session:          false,
		Following:        false,
		DisableUrlEncode: false,
		Analyze:          false,
		Retry:            1,
		ReadTimeout:      0,
		MaxBodySize:      0,
	}

	url := getURL(rc)

	r := client.Get(url, this.header)

	if !r.Success {
		return rp
	}

	rp.CName = r.CommonName
	rp.Banner = headerToString(r.Headers, r.Proto, r.Status) + r.Text
	rp.Domains = appendDomains(rp.Domains, extractDomains(r.Text))

	resA := this.w.Analyze(r)
	apps := resA.R
	for _, app := range apps {
		if rp.IsSetApp(app.AppName) {
			continue
		}
		rp.Apps = append(rp.Apps, dbio.WebApp{
			AppName: app.AppName,
			Version: app.Version,
			Implies: app.Implies,
		})
	}

	client.Following = true

	r = client.Get(url, this.header)
	resA = this.w.Analyze(r)
	apps = resA.R
	for _, app := range apps {
		if rp.IsSetApp(app.AppName) {
			continue
		}
		rp.Apps = append(rp.Apps, dbio.WebApp{
			AppName: app.AppName,
			Version: app.Version,
			Implies: app.Implies,
		})
	}
	rp.Domains = appendDomains(rp.Domains, extractDomains(r.Text))

	favLink := resA.Icon
	if favLink != "" {
		if httpLink.MatchString(favLink) {
			r = client.Get(favLink, nil)
			if r.Success {
				favSum := fmt.Sprintf("%x", md5.Sum([]byte(r.Text)))
				rp.Favicon = favSum
			}
		} else {
			if !strings.HasPrefix(favLink, "/") {
				favLink = "/" + favLink
			}
			r = client.Get(url+favLink, nil)
			if r.Success {
				favSum := fmt.Sprintf("%x", md5.Sum([]byte(r.Text)))
				rp.Favicon = favSum
			}
		}
	} else {
		r = client.Get(url+"/favicon.ico", nil)
		if r.Success {
			favSum := fmt.Sprintf("%x", md5.Sum([]byte(r.Text)))
			rp.Favicon = favSum
		}
	}

	r = client.Get(url+"/console", this.header)
	resA = this.w.Analyze(r)
	apps = resA.R
	for _, app := range apps {
		if rp.IsSetApp(app.AppName) {
			continue
		}
		rp.Apps = append(rp.Apps, dbio.WebApp{
			AppName: app.AppName,
			Version: app.Version,
			Implies: app.Implies,
		})
	}

	return rp
}

func (this *engine) worker() {
	go func() {
		for {
			j := <-this.in

			res := this.scan(j)

			if res.Service == "" {
				continue
			}
			this.out <- res
		}
	}()
}
