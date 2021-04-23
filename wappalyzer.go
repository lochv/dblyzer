package dblyzer

import (
	"bytes"
	"dblyzer/internal/httpclient"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type appsDefinition struct {
	Apps map[string]app      `json:"apps"`
	Cats map[string]category `json:"categories"`
}
type Results struct {
	R []Result
	Icon string
}
type Result struct {
	icon       string
	Categories []string
	AppName    string
	Version    string
	Implies    []string
}
type category struct {
	Name string `json:"name"`
}

type Match struct {
	app     `json:"app"`
	AppName string     `json:"appname"`
	Matches [][]string `json:"matches"`
	Version string     `json:"version"`
}

func (m *Match) updateVersion(version string) {
	if version != "" {
		m.Version = version
	}
}

type app struct {
	Cats     stringArray       `json:"cats"`
	CatNames []string          `json:"category_names"`
	Cookies  map[string]string `json:"cookies"`
	Headers  map[string]string `json:"headers"`
	Meta     map[string]string `json:"meta"`
	HTML     stringArray       `json:"html"`
	Script   stringArray       `json:"script"`
	URL      stringArray       `json:"url"`
	Website  string            `json:"website"`
	Implies  stringArray       `json:"implies"`

	hTMLRegex   []appRegexp `json:"-"`
	scriptRegex []appRegexp `json:"-"`
	uRLRegex    []appRegexp `json:"-"`
	headerRegex []appRegexp `json:"-"`
	metaRegex   []appRegexp `json:"-"`
	cookieRegex []appRegexp `json:"-"`
}

func (app *app) findInHeaders(headers http.Header) (matches [][]string, version string) {
	var v string

	for _, hre := range app.headerRegex {
		if headers.Get(hre.Name) == "" {
			continue
		}
		hk := http.CanonicalHeaderKey(hre.Name)
		for _, headerValue := range headers[hk] {
			if headerValue == "" {
				continue
			}
			if m, version := findMatches(headerValue, []appRegexp{hre}); len(m) > 0 {
				matches = append(matches, m...)
				v = version
			}
		}
	}
	return matches, v
}

type stringArray []string

func (t *stringArray) UnmarshalJSON(data []byte) error {
	var s string
	var sa []string
	var na []int

	if err := json.Unmarshal(data, &s); err != nil {
		if err := json.Unmarshal(data, &na); err == nil {
			// not a string, so maybe []int?
			*t = make(stringArray, len(na))

			for i, number := range na {
				(*t)[i] = fmt.Sprintf("%d", number)
			}

			return nil
		} else if err := json.Unmarshal(data, &sa); err == nil {
			// not a string, so maybe []string?
			*t = sa
			return nil
		}
		fmt.Println(string(data))
		return err
	}
	*t = stringArray{s}
	return nil
}

type appRegexp struct {
	Name    string
	Regexp  *regexp.Regexp
	Version string
}

type Wappalyzer struct {
	appDefs *appsDefinition
	in      chan input
	out     chan []Result
}

type input struct {
	r   httpclient.Response
	res chan Results
}

func newWappalyzer(filePath string, worker int) *Wappalyzer {
	f, err := os.Open(filePath)
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()
	var appDefs *appsDefinition
	dec := json.NewDecoder(f)
	if err = dec.Decode(&appDefs); err != nil {
		panic(err.Error())
	}

	for key, value := range appDefs.Apps {

		app := appDefs.Apps[key]

		app.hTMLRegex = compileRegexes(value.HTML)
		app.scriptRegex = compileRegexes(value.Script)
		app.uRLRegex = compileRegexes(value.URL)

		app.headerRegex = compileNamedRegexes(app.Headers)
		app.metaRegex = compileNamedRegexes(app.Meta)
		app.cookieRegex = compileNamedRegexes(app.Cookies)

		app.CatNames = make([]string, 0)

		for _, cid := range app.Cats {
			if category, ok := appDefs.Cats[string(cid)]; ok && category.Name != "" {
				app.CatNames = append(app.CatNames, category.Name)
			}
		}

		appDefs.Apps[key] = app
	}
	w := &Wappalyzer{appDefs: appDefs, in: make(chan input, 100)}

	for i := 0; i < worker; i++ {
		w.run()
	}

	return w
}

func (w *Wappalyzer) run() {
	go func() {
		for {
			select {
			case r := <-w.in:
				w.analyze(r)
			}
		}
	}()
}

func (w *Wappalyzer) Analyze(r httpclient.Response) Results {
	res := make(chan Results)
	w.in <- input{r, res}
	return <-res
}

func (w *Wappalyzer) analyze(r input) {
	var results []Result
	results = make([]Result, 0)
	icon, matched := w.process(r.r)
	for index := range matched {
		result := Result{
			Categories: nil,
			AppName:    "",
			Version:    "",
			Implies:    nil,
		}
		result.Categories = matched[index].CatNames
		result.AppName = matched[index].AppName
		result.Version = matched[index].Version
		result.Implies = matched[index].Implies
		results = append(results, result)
	}
	select {
	case r.res <- Results{
		R:    results,
		Icon: icon,
	}:
		return
	case <-time.After(5 * time.Second):
		return
	}
}

func (w *Wappalyzer) process(r httpclient.Response) (string, []Match) {
	var canParseBody = true
	var icon = ""
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(r.Text)))
	if err != nil {
		canParseBody = false
	}

	apps := make([]Match, 0)
	cookiesMap := make(map[string]string)

	for _, c := range r.Cookies {
		cookiesMap[c.Name] = c.Value
	}

	// handle crawling
	for Appname, app := range w.appDefs.Apps {
		// TODO: Reduce complexity in this for-loop by functionalising out
		// the sub-loops and checks.

		findings := Match{
			app:     app,
			AppName: Appname,
			Matches: make([][]string, 0),
		}

		// check uri

		if m, v := findMatches(r.Path, app.uRLRegex); len(m) > 0 && r.StatusCode == 200 {
			findings.Matches = append(findings.Matches, m...)
			findings.updateVersion(v)
		}

		// check response header
		headerFindings, version := app.findInHeaders(r.Headers)
		findings.Matches = append(findings.Matches, headerFindings...)
		findings.updateVersion(version)

		// check cookies
		for _, c := range app.cookieRegex {
			if _, ok := cookiesMap[c.Name]; ok {

				// if there is a regexp set, ensure it matches.
				// otherwise just add this as a match
				if c.Regexp != nil {

					// only match single AppRegexp on this specific cookie
					if m, v := findMatches(cookiesMap[c.Name], []appRegexp{c}); len(m) > 0 {
						findings.Matches = append(findings.Matches, m...)
						findings.updateVersion(v)
					}

				} else {
					findings.Matches = append(findings.Matches, []string{c.Name})
				}
			}

		}
		if canParseBody {
			// get icon

			doc.Find("link").Each(func(i int, s *goquery.Selection) {
				if script, exists := s.Attr("href"); exists {
					if ic, exists := s.Attr("rel") ; exists {
						if strings.ToLower(ic) == "icon" || strings.ToLower(ic) == "shortcut icon" {
							icon = strings.Replace(script, "../", "/", -1)
						}
					}
				}
			})

			// check raw html
			if m, v := findMatches(r.Text, app.hTMLRegex); len(m) > 0 {
				findings.Matches = append(findings.Matches, m...)
				findings.updateVersion(v)
			}

			// check script tags
			doc.Find("script").Each(func(i int, s *goquery.Selection) {
				if script, exists := s.Attr("src"); exists {
					if m, v := findMatches(script, app.scriptRegex); len(m) > 0 {
						findings.Matches = append(findings.Matches, m...)
						findings.updateVersion(v)
					}
				}
			})

			// check meta tags
			for _, h := range app.metaRegex {
				selector := fmt.Sprintf("meta[name='%s']", h.Name)
				doc.Find(selector).Each(func(i int, s *goquery.Selection) {
					content, _ := s.Attr("content")
					if m, v := findMatches(content, []appRegexp{h}); len(m) > 0 {
						findings.Matches = append(findings.Matches, m...)
						findings.updateVersion(v)
					}
				})
			}
		}

		if len(findings.Matches) > 0 {
			apps = append(apps, findings)

			// handle implies
			for _, implies := range app.Implies {
				for implyAppname, implyApp := range w.appDefs.Apps {
					if implies != implyAppname {
						continue
					}

					f2 := Match{
						app:     implyApp,
						AppName: implyAppname,
						Matches: make([][]string, 0),
					}
					apps = append(apps, f2)
				}

			}
		}
	}
	return icon, apps
}
