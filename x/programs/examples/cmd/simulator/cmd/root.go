package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	defaultDatabase = ".simulator"
)

func init() {
	cobra.EnablePrefixMatching = true
	rootCmd.AddCommand(
		programCmd,
	)

	programCmd.AddCommand(
		programCreateCmd,
	)

	rootCmd.PersistentFlags().StringVar(
		&dbPath,
		"database",
		defaultDatabase,
		"path to database (will create if missing)",
	)

	rootCmd.PersistentPreRunE = func(*cobra.Command, []string) (err error) {
		utils.Outf("{{yellow}}database:{{/}} %s\n", dbPath)
		db, _, err = pebble.New(dbPath, pebble.NewDefaultConfig())
		return err
	}

	rootCmd.PersistentPostRunE = func(*cobra.Command, []string) error {
		return db.Close()
	}

	programCreateCmd.PersistentFlags().StringVar(
		&programName,
		"name",
		"",
		"name of the program",
	)

}

var (
	programName      string
	dbPath           string
	db               database.Database
	logLevel         string
	programPath      string
	rustPath         string
	profilingEnabled bool
	meteringEnabled  bool
	rootCmd          = &cobra.Command{
		Use:   "simulator",
		Short: "HyperSDK program simulator",
	}
)

func Execute() error {
	return rootCmd.Execute()
}
