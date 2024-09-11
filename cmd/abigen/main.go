// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/abi"
)

var (
	packageName string
	rootCmd     = &cobra.Command{
		Use:   "abigen <input_abi.json> <output_file.go>",
		Short: "Generate Go structs from ABI JSON",
		Args:  cobra.ExactArgs(2),
		Run:   run,
	}
)

func init() {
	rootCmd.Flags().StringVarP(&packageName, "package", "p", "", "Package name for generated code (overrides default)")
}

func run(_ *cobra.Command, args []string) {
	inputFile := args[0]
	outputFile := args[1]

	// Read the input ABI JSON file
	abiData, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Parse the ABI JSON
	var vmABI abi.VM
	err = json.Unmarshal(abiData, &vmABI)
	if err != nil {
		fmt.Printf("Error parsing ABI JSON: %v\n", err)
		os.Exit(1)
	}

	if packageName == "" {
		packageName = filepath.Base(filepath.Dir(outputFile))
	}

	// Generate Go structs
	generatedCode, err := abi.GenerateGoStructs(vmABI, packageName)
	if err != nil {
		fmt.Printf("Error generating Go structs: %v\n", err)
		os.Exit(1)
	}

	// Create the directory for the output file if it doesn't exist
	outputDir := filepath.Dir(outputFile)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Write the generated code to the output file
	err = os.WriteFile(outputFile, []byte(generatedCode), 0o600)
	if err != nil {
		fmt.Printf("Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated Go structs in %s\n", outputFile)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
