package main

import (
	_ "embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"sort"
	"strings"
)

//go:embed results.html
var results_tpl string

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

// QueryHost performs net.LookupHost on a host name and return the responses in distinct IPv4 and IPv6 lists.
func QueryHost(host string) ([]string, []string, error) {
	var a4, a6 []string

	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, nil, err
	}
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

// check if given error is the DNS IsNotFound error
func isNotfound(err error) bool {
	dnserr, ok := err.(*net.DNSError)
	if ok {
		return dnserr.IsNotFound
	}
	return false
}

// QueryHost performs various DNS lookups to fill in the Result struct
func QueryDomain(domain string) (*Result, error) {

	domain = strings.TrimPrefix(domain, "www.")
	var r Result = Result{Domain: domain}
	var err error
	// hosts
	r.Host4, r.Host6, err = QueryHost(domain)
	if err != nil && !isNotfound(err) {
		return nil, fmt.Errorf("QueryHost failed: %v", err)
	}
	// wwww hosts
	r.WWW4, r.WWW6, err = QueryHost("www." + domain)
	if err != nil && !isNotfound(err) {
		return nil, fmt.Errorf("QueryHost (www) failed: %v", err)
	}
	// NS
	nss, err := net.LookupNS(domain)
	if err != nil && !isNotfound(err) {
		return nil, fmt.Errorf("LookupNS failed: %v", err)
	}
	// no NS at , nor IPv4 nor IPv6 (shouldn't happen)
	if len(nss) == 0 {
		return nil, fmt.Errorf("LookupNS failed: domain has no NS")
	}
	for _, ns := range nss {
		ns4, ns6, err := QueryHost(ns.Host)
		if err != nil {
			return nil, fmt.Errorf("QueryHost for NS failed: %v", err)
		}
		r.NS4 = append(r.NS4, ns4...)
		r.NS6 = append(r.NS6, ns6...)
	}
	// MX
	mxs, err := net.LookupMX(domain)
	if err != nil && !isNotfound(err) {
		return nil, fmt.Errorf("LookupMX failed: %v", err)
	}
	for _, mx := range mxs {
		mx4, mx6, err := QueryHost(mx.Host)
		if err != nil && !isNotfound(err) {
			return nil, fmt.Errorf("QueryHost for MX failed: %v", err)
		} else {
			r.MX4 = append(r.MX4, mx4...)
			r.MX6 = append(r.MX6, mx6...)
		}
	}

	// sort all lists
	sort.Strings(r.Host4)
	sort.Strings(r.Host6)
	sort.Strings(r.WWW4)
	sort.Strings(r.WWW6)
	sort.Strings(r.NS4)
	sort.Strings(r.NS6)
	sort.Strings(r.MX4)
	sort.Strings(r.MX6)

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

func Rank(r *Result) string {
	if r == nil {
		return "?????"
	}
	stars := 0
	if len(r.Host6) > 0 {
		stars += 1
	}
	if len(r.MX4) > 0 && len(r.MX6) > 0 {
		stars += 1
	}
	if len(r.WWW4) > 0 && len(r.WWW6) > 0 {
		stars += 1
	}
	if len(r.NS6) > 0 {
		// NYI: r.DNS6Only so we +2 if NS v6
		stars += 2
	}

	return strings.Repeat("*", stars)
}

func main() {

	addr := flag.String("a", ":3000", "address to listen to. format = [address]:port ")
	verbose := flag.Bool("v", false, "verbose output")
	flag.Parse()

	if flag.NArg() == 0 {
		server(*addr)
		os.Exit(0)
	}

	for _, s := range flag.Args() {
		r, err := QueryDomain(s)
		if err != nil {
			log.Fatal(err)
		}
		if *verbose {
			r.Display()
		} else {
			fmt.Printf("%s : %s\n", r.Domain, Rank(r))
		}
	}
}

func server(addr string) {
	tpl, err := template.New("page").Parse(results_tpl)
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
	fmt.Printf("start listening on %s (ctrl-c to quit)\n", addr)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}

}
