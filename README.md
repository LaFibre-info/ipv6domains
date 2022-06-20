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

## dev
use `-web path` to use a local web root path instead of the embeded fs (handy to live reload modifications to the templates / html files)