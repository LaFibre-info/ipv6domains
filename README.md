# ipv6domains

A basic POC of reproducing [ip6.nl](https://ip6.nl) with Go.

## build and install

    go install github.com/LaFibre-info/ipv6domains@latest

or clone the repo and `go run main.go`
## usage
launching without argument will start a HTTP server listening in port 3000.

Use `-a [address]:port` to change the port and/or the adress to listen to (example: `ipv6domains -a :8080`)

If at least one argument is passed, the program will work in CLI mode and display the result for the argument(s) (examples: `ipv6domains google.com`)
