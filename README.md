# ipv6domains

IPv6 readiness tester for DNS domains

Currently just a basic POC of reproducing [ip6.nl](https://ip6.nl) with Go.

## build and install

    go install github.com/LaFibre-info/ipv6domains@latest

or clone the repo and `go run main.go`
## usage
launching `ipv6domains` without argument will start a HTTP server listening on port 3000.

Use `-a [address]:port` to change the port and/or the address to listen on (example: `ipv6domains -a :8080`)

If at least one argument is passed, the program will work in CLI mode and will display the result for the argument(s) (examples: `ipv6domains google.com`)

if `-check mode` is passed, the program runs in batch mode and expect to read a list of domain names from the standard input. 
It will then process each domain name based on the `mode` value:
  * 4 : only output the domain names that are IPv4 only (A or www A entries and no AAAA or www AAAA entries)
  * 6 : only output the domain names that are IPv6 only (no A or www A entries and AAAA or www AAAA entries)
  * 1 : only output the domain names that have errors
  * any other value: output number of entries for the following: A, A for www, AAAA, AAAA for www
This is done in parallel using max `njobs` workers at a time (use `-njobs n` to change this)
## dev
use `-web path` to use a local web root path instead of the embeded fs (handy to live reload modifications to the templates / html files)