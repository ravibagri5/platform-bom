// Command gen refreshes the embedded CNCF landscape snapshot: go generate ./internal/landscape
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/ravibagri5/platform-bom/internal/landscape"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	resp, err := http.Get(landscape.DefaultURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", landscape.DefaultURL, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	projects, err := landscape.Parse(data)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(projects, "", " ")
	if err != nil {
		return err
	}
	if err := os.WriteFile("cncf.json", append(out, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %d CNCF projects to cncf.json\n", len(projects))
	return nil
}
