package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/db"
)

func newBackupCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "backup <dest>",
		Short: "Write a consistent copy of the database to dest (must not exist)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			dest := args[0]
			if _, err := os.Stat(dest); err == nil {
				return fmt.Errorf("backup: %s already exists", dest)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("backup: %w", err)
			}
			if _, err := os.Stat(opts.dbPath); err != nil {
				return fmt.Errorf("backup: database %s: %w", opts.dbPath, err)
			}
			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			if err := db.Backup(ctx, sqlDB, dest); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backup written to %s (integrity check ok)\n", dest)
			return nil
		},
	}
}
