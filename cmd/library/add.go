package library

import (
	"fmt"
	"path/filepath"

	"github.com/ronelliott/muzak/library"
	"github.com/ronelliott/snek"
	"github.com/spf13/cobra"
)

func newAddCommand() (*cobra.Command, error) {
	return snek.NewCommand(
		snek.WithUse("add <path>"),
		snek.WithShort("Add a source directory to the library"),
		snek.WithSimpleRunE(func(args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("expected exactly one path argument")
			}

			abs, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			fmt.Printf("Scanning %s…\n", abs)
			if _, err := library.Scan([]string{abs}); err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			sources := library.LoadSources()
			if err := sources.Add(abs); err != nil {
				return err
			}
			if err := sources.Save(); err != nil {
				return fmt.Errorf("save sources: %w", err)
			}

			fmt.Printf("Added %s\n", abs)
			return nil
		}),
	)
}
