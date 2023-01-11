package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/julienschmidt/httprouter"
)

//go:embed web/*
var webDir embed.FS

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

// func customResolver() {
// 	r := &net.Resolver{
// 		PreferGo: true,
// 		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
// 			d := net.Dialer{
// 				Timeout: time.Millisecond * time.Duration(10000),
// 			}
// 			return d.DialContext(ctx, network, "8.8.8.8:53")
// 		},
// 	}
// 	ip, _ := r.LookupHost(context.Background(), "www.google.com")
// }

// QueryHost performs net.LookupHost on a host name and return the responses in distinct IPv4 and IPv6 lists.
func QueryHost(host string) (ipv4 []string, ipv6 []string, err error) {
	//var a4, a6 []string

	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, nil, err
	}
	for _, h := range addrs {
		a, _ := netip.ParseAddr(h)
		if a.Is4() {
			ipv4 = append(ipv4, a.String())
		}
		if a.Is6() {
			ipv6 = append(ipv6, a.String())
		}
	}
	return ipv4, ipv6, nil
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
	verbose := flag.Bool("v", false, "verbose output (for cmd line mode)")
	web := flag.String("web", "", "use local web directoy instead of embeded content")
	check := flag.Int("check", 0, "check domain names from stdin (cmd line mode only)")
	njobs := flag.Int("njobs", 5, "number of jobs for check domains (cmd line mode only, requires -check)")

	flag.Parse()

	f, err := fs.Sub(webDir, "web")
	if err != nil {
		log.Fatal(err)
	}

	reparse := false
	if *web != "" {
		f = os.DirFS(*web)
		reparse = true
	}

	if *check != 0 {
		scanFile(os.Stdin, *check, *njobs)
		os.Exit(0)
	}

	if flag.NArg() == 0 {
		server(*addr, f, reparse)
		os.Exit(0)
	}

	for _, s := range flag.Args() {
		r, err := QueryDomain(s)
		if err != nil {
			fmt.Printf("%s: error %s\n", s, err)
			continue
		}
		if *verbose {
			r.Display()
		}
		fmt.Printf("%s : %s\n", r.Domain, Rank(r))
	}
}

func parseTpl(fs fs.FS, path string) (*template.Template, error) {
	tpl, err := template.ParseFS(fs, path)
	if err != nil {
		return nil, err
	}
	return tpl.Lookup("page"), nil
}

func server(addr string, fs fs.FS, reparse bool) {

	t, err := parseTpl(fs, "templates/*.html")
	if err != nil {
		log.Fatal(err)
	}
	hdl := func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		// should sanatize
		result, err := QueryDomain(ps.ByName("domain"))
		fmt.Printf("%s asked for domain = %s\n", r.RemoteAddr, ps.ByName("domain"))
		if err != nil {
			http.Error(w, fmt.Sprintf("QueryDomain error: %v", err), http.StatusInternalServerError)
			return
		}

		//result.Display()
		if reparse {
			t, err = parseTpl(fs, "templates/*.html")
			//fmt.Printf("reparsed\n")
			if err != nil {
				http.Error(w, fmt.Sprintf("parseTpl error: %v", err), http.StatusInternalServerError)
				return
			}
		}
		err = t.Execute(w, result)
		if err != nil {
			http.Error(w, fmt.Sprintf("QueryDomain error: %v", err), http.StatusInternalServerError)
			return
		}
	}

	router := httprouter.New()
	router.GET("/q/:domain", hdl)
	router.NotFound = http.FileServer(http.FS(fs))

	fmt.Printf("start listening on %s (ctrl-c to quit)\n", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}

// scan a file,reading domain names, one by line
// and apply a check based on the "mode" argument value:
// 4 - only output the domain names that are IPv4 only
// 6 - only output the domain names that are IPv6 only
// any other value: out #IP entries
// The check is parallelized with numjobs checks at a time
func scanFile(file *os.File, mode int, numjobs int) {
	scan := bufio.NewScanner(file)
	if numjobs <= 0 {
		log.Fatal("scanFile: wrong number of jobs")
	}

	domains := make(chan string, numjobs)
	var wg sync.WaitGroup
	for w := 1; w <= numjobs; w++ {
		wg.Add(1)
		go func(doms <-chan string, wn int) {
			defer wg.Done()
			for dom := range doms {
				//fmt.Printf("worker %d - checking %s\n", wn, dom)
				checkDom(dom, mode)
			}
		}(domains, w)
	}

	for scan.Scan() {
		s := scan.Text()
		domains <- s
	}
	close(domains)
	wg.Wait()
}

// checkDom applies a check on a domain based on the "mode" argument value:
// 4 - only output the domain names that are IPv4 only
// 6 - only output the domain names that are IPv6 only
// any other value: out #IP entries
func checkDom(dom string, mode int) {
	r, err := QueryDomain(dom)
	if err != nil {
		fmt.Printf("%s, (%s)\n", dom, err)
		return
	}
	if mode != 4 && mode != 6 && mode != 1 {
		fmt.Printf("%s, %d, %d, %d, %d\n", r.Domain, len(r.Host4), len(r.WWW4), len(r.Host6), len(r.WWW6))
		return
	}

	ip4 := len(r.Host4)+len(r.WWW4) > 0
	ip6 := len(r.Host6)+len(r.WWW6) > 0
	if mode == 6 && ip6 && !ip4 {
		fmt.Printf("%s\n", r.Domain)
		return
	}
	if mode == 4 && !ip6 && ip4 {
		fmt.Printf("%s\n", r.Domain)
		return
	}
}
