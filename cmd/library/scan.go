package library

import (
	"fmt"

	"github.com/ronelliott/muzak/library"
	"github.com/ronelliott/snek"
	"github.com/spf13/cobra"
)

func newScanCommand() (*cobra.Command, error) {
	return snek.NewCommand(
		snek.WithUse("scan"),
		snek.WithShort("Re-scan all configured sources and update the cache"),
		snek.WithSimpleRunE(func(args []string) error {
			sources := library.LoadSources()
			if len(sources.Paths) == 0 {
				return fmt.Errorf("no sources configured — use 'muzak library add <path>' first")
			}

			fmt.Printf("Scanning %d source(s)…\n", len(sources.Paths))
			tracks, err := library.Scan(sources.Paths)
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			fmt.Printf("Found %d track(s)\n", len(tracks))
			return nil
		}),
	)
}
