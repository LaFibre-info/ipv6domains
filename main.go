package main

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"unicode/utf8"
)

type Result struct {
	Domain   string
	Host4    []string
	Host6    []string
	NS4      []string
	NS6      []string
	DNS6Only bool
	MX4      []string
	MX6      []string
	WWW4     []string
	WWW6     []string
}

// from https://gist.github.com/chmike/d4126a3247a6d9a70922fc0e8b4f4013
// checkDomain returns an error if the domain name is not valid
// See https://tools.ietf.org/html/rfc1034#section-3.5 and
// https://tools.ietf.org/html/rfc1123#section-2.
func checkDomain(name string) error {
	switch {
	case len(name) == 0:
		return fmt.Errorf("check domain: empty domain")
	case len(name) > 255:
		return fmt.Errorf("check domain: name length is %d, can't exceed 255", len(name))
	}
	var l int
	for i := 0; i < len(name); i++ {
		b := name[i]
		if b == '.' {
			// check domain labels validity
			switch {
			case i == l:
				return fmt.Errorf("check domain: invalid character '%c' at offset %d: label can't begin with a period", b, i)
			case i-l > 63:
				return fmt.Errorf("check domain: byte length of label '%s' is %d, can't exceed 63", name[l:i], i-l)
			case name[l] == '-':
				return fmt.Errorf("check domain: label '%s' at offset %d begins with a hyphen", name[l:i], l)
			case name[i-1] == '-':
				return fmt.Errorf("check domain: label '%s' at offset %d ends with a hyphen", name[l:i], l)
			}
			l = i + 1
			continue
		}
		// test label character validity, note: tests are ordered by decreasing validity frequency
		if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-' || b >= 'A' && b <= 'Z') {
			// show the printable unicode character starting at byte offset i
			c, _ := utf8.DecodeRuneInString(name[i:])
			if c == utf8.RuneError {
				return fmt.Errorf("check domain: invalid rune at offset %d", i)
			}
			return fmt.Errorf("check domain: invalid character '%c' at offset %d", c, i)
		}
	}
	// check top level domain validity
	switch {
	case l == len(name):
		return fmt.Errorf("check domain: missing top level domain, domain can't end with a period")
	case len(name)-l > 63:
		return fmt.Errorf("check domain: byte length of top level domain '%s' is %d, can't exceed 63", name[l:], len(name)-l)
	case name[l] == '-':
		return fmt.Errorf("check domain: top level domain '%s' at offset %d begins with a hyphen", name[l:], l)
	case name[len(name)-1] == '-':
		return fmt.Errorf("check domain: top level domain '%s' at offset %d ends with a hyphen", name[l:], l)
	case name[l] >= '0' && name[l] <= '9':
		return fmt.Errorf("check domain: top level domain '%s' at offset %d begins with a digit", name[l:], l)
	}
	return nil
}

// QueryHost performs net.LookupHost on a host name and return the responses in distinct IPv4 and IPv6 lists.
func QueryHost(host string) ([]string, []string, error) {
	var a4, a6 []string

	addrs, _ := net.LookupHost(host)
	for _, h := range addrs {
		a, _ := netip.ParseAddr(h)
		if a.Is4() {
			a4 = append(a4, a.String())
		}
		if a.Is6() {
			a6 = append(a6, a.String())
		}
	}
	return a4, a6, nil
}

// QueryHost performs various DNS lookups to fill in the Result struct
func QueryDomain(domain string) (*Result, error) {

	domain = strings.TrimPrefix(domain, "www.")
	var r Result = Result{Domain: domain}

	// hosts
	r.Host4, r.Host6, _ = QueryHost(domain)

	// wwww hosts
	r.WWW4, r.WWW6, _ = QueryHost("www." + domain)

	// NS
	nss, err := net.LookupNS(domain)
	if err != nil {
		return nil, fmt.Errorf("LookupNS failed: %v", err)
	}
	for _, ns := range nss {
		ns4, ns6, _ := QueryHost(ns.Host)
		r.NS4 = append(r.NS4, ns4...)
		r.NS6 = append(r.NS6, ns6...)
	}
	// MX
	mxs, err := net.LookupMX(domain)
	if err != nil {
		return nil, fmt.Errorf("LookupMX failed: %v", err)
	}
	for _, mx := range mxs {
		mx4, mx6, _ := QueryHost(mx.Host)
		r.MX4 = append(r.MX4, mx4...)
		r.MX6 = append(r.MX6, mx6...)
	}
	return &r, nil
}

func (r *Result) Display() {
	fmt.Printf("result for %s:\n", r.Domain)

	fmt.Printf("IPv4:\n")
	for _, s := range r.Host4 {
		fmt.Printf("  %s\n", s)
	}
	fmt.Printf("IPv6:\n")
	for _, s := range r.Host6 {
		fmt.Printf("  %s\n", s)
	}

	fmt.Printf("wwww IPv4:\n")
	for _, s := range r.WWW4 {
		fmt.Printf("  %s\n", s)
	}
	fmt.Printf("wwww IPv6:\n")
	for _, s := range r.WWW6 {
		fmt.Printf("  %s\n", s)
	}

	fmt.Printf("DNS Servers IPv4:\n")
	for _, s := range r.NS4 {
		fmt.Printf("  %s\n", s)
	}
	fmt.Printf("DNS Servers IPv6:\n")
	for _, s := range r.NS6 {
		fmt.Printf("  %s\n", s)
	}
	fmt.Printf("Mail exchangers IPv4:\n")
	for _, s := range r.MX4 {
		fmt.Printf("  %s\n", s)
	}
	fmt.Printf("Mail exchangers IPv6:\n")
	for _, s := range r.MX6 {
		fmt.Printf("  %s\n", s)
	}

}
func main() {
	if len(os.Args) == 1 {
		server()
		os.Exit(0)
	}

	for _, s := range os.Args[1:2] {
		r, err := QueryDomain(s)
		if err != nil {
			log.Fatal(err)
		}
		r.Display()
	}
}

func server() {
	tpl, err := template.New("page").ParseFiles("./results.html")
	if err != nil {
		log.Fatal(err)
	}
	t := tpl.Lookup("page")

	hdl := func(w http.ResponseWriter, r *http.Request) {

		// should sanatize
		result, err := QueryDomain(strings.TrimPrefix(r.URL.Path, "/"))

		if err != nil {
			http.Error(w, fmt.Sprintf("QueryDomain error: %v", err), http.StatusInternalServerError)
			return
		}

		//result.Display()

		err = t.Execute(w, result)
		if err != nil {
			http.Error(w, fmt.Sprintf("QueryDomain error: %v", err), http.StatusInternalServerError)
		}
	}

	http.HandleFunc("/", hdl)
	http.ListenAndServe(":3000", nil)
}
