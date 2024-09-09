// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/hypersdk/abi"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: hypersdk-abi-codegen <input_abi.json> <output_file.go>")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]

	// Read the input ABI JSON file
	abiData, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Parse the ABI JSON
	var vmABI abi.VMABI
	err = json.Unmarshal(abiData, &vmABI)
	if err != nil {
		fmt.Printf("Error parsing ABI JSON: %v\n", err)
		os.Exit(1)
	}

	packageName := filepath.Base(filepath.Dir(outputFile))

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
