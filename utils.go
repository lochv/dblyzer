package dblyzer

import (
	"bytes"
	"dblyzer/internal/dbio"
	"dblyzer/internal/pcre"
	"fmt"
	"mvdan.cc/xurls/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var httpRegex, _ = pcre.Compile(`http`, pcre.ANCHORED)
var httpsRegex, _ = pcre.Compile(`https|((?>ssl)(.*)(?>http))`, pcre.ANCHORED)
var faviconRegex = regexp.MustCompile(`(?im)<link.*?icon.*?href=('|")(.*)('|")|<link.*?href=('|")(.*)('|").*?icon`)
var linkRegex = regexp.MustCompile(`(?im)<link[^<]+href=('|")[^<]+>`)
var httpLink = regexp.MustCompile(`http.*?`)
var rxStrict = xurls.Strict()

func compileRegexes(s stringArray) []appRegexp {
	var list []appRegexp

	for _, regexString := range s {

		// Split version detection
		splitted := strings.Split(regexString, "\\;")

		regex, err := regexp.Compile(splitted[0])
		if err != nil {
		} else {
			rv := appRegexp{
				Regexp: regex,
			}

			if len(splitted) > 1 && strings.HasPrefix(splitted[0], "version") {
				rv.Version = splitted[1][8:]
			}

			list = append(list, rv)
		}
	}

	return list
}

func compileNamedRegexes(from map[string]string) []appRegexp {

	var list []appRegexp

	for key, value := range from {

		h := appRegexp{
			Name: key,
		}

		if value == "" {
			value = ".*"
		}

		// Filter out webapplyzer attributes from regular expression
		splitted := strings.Split(value, "\\;")

		r, err := regexp.Compile(splitted[0])
		if err != nil {
			continue
		}

		if len(splitted) > 1 && strings.HasPrefix(splitted[1], "version:") {
			h.Version = splitted[1][8:]
		}

		h.Regexp = r
		list = append(list, h)
	}

	return list
}

func findVersion(matches [][]string, version string) string {
	var v string

	for _, matchPair := range matches {
		// replace backtraces (max: 3)
		for i := 1; i <= 3; i++ {
			bt := fmt.Sprintf("\\%v", i)
			if strings.Contains(version, bt) && len(matchPair) >= i {
				v = strings.Replace(version, bt, matchPair[i], 1)
			}
		}

		// return first found version
		if v != "" {
			return v
		}

	}

	return ""
}

func findMatches(content string, regexes []appRegexp) ([][]string, string) {
	var m [][]string
	var version string

	for _, r := range regexes {
		matches := r.Regexp.FindAllStringSubmatch(content, -1)
		if matches == nil {
			continue
		}

		m = append(m, matches...)

		if r.Version != "" {
			version = findVersion(m, r.Version)
		}
	}
	return m, version
}

func isHTTP(serviceName string) bool {
	if httpRegex.FindIndex([]byte(serviceName), 0) == nil {
		return false
	}
	return true
}

func isHTTPS(serviceName string) bool {
	if httpsRegex.FindIndex([]byte(serviceName), 0) == nil {
		return false
	}
	return true
}

func getFaviconlink(page string) string {
	links := linkRegex.FindAllStringSubmatch(page, 20)
	for _, link := range links {
		fav := faviconRegex.FindStringSubmatch(link[0])
		if fav != nil && len(link) > 1 {
			hlink := strings.Split(fav[0], `href=`+link[1])
			if len(hlink) > 1 {
				return strings.Replace(strings.Split(hlink[1], link[1])[0], "../", "/", -1)
			}
		}
	}
	return ""
}

func extractDomains(page string) []string {
	var domains []string
	urls := rxStrict.FindAllString(page, -1)
	for _, re := range urls {
		u, err := url.Parse(re)
		if err != nil {
			continue
		}
		for _, domain := range domains {
			if domain == u.Host {
				continue
			}
		}
		domains = append(domains, u.Host)
	}
	return domains
}

func appendDomains(slice []string, elem []string) []string {

	for _, e := range elem {
		var isset = false
		for _, s := range slice {
			if e == s {
				isset = true
				break
			}
		}
		if isset == false {
			slice = append(slice, e)
		}
	}

	return slice
}

func getURL(rc dbio.Receive) string {
	if (rc.Service == "https" && rc.Port == 443) || (rc.Service == "http" && rc.Port == 80) {
		return rc.Service + "://" + rc.Host
	}
	return rc.Service + "://" + rc.Host + ":" + strconv.Itoa(rc.Port)
}

func headerToString(m http.Header, proto string, status string) string {
	b := new(bytes.Buffer)
	fmt.Fprintf(b, "%s %s\n", proto, status)
	for key, value := range m {
		fmt.Fprintf(b, "%s:%s\n", key, value[0])
	}
	fmt.Fprintf(b, "\n")
	return b.String()
}
