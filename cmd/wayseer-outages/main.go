// Command wayseer-outages serves the outages module from its own process, for the app to run
// from a signed package; `make sign` builds and signs it.
package main

import (
	"github.com/wayseer-net/desktop-module-outages"
	"wayseer.dev/sdk/serve"
)

func main() { serve.Main(outages.New()) }
