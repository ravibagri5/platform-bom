// Command pbom discovers, versions and presents an internal platform as a product.
package main

import (
	"os"

	"github.com/ravibagri5/platform-bom/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
