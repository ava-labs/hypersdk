package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

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
	abiData, err := ioutil.ReadFile(inputFile)
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

	// Generate Go structs
	generatedCode, err := abi.GenerateGoStructs(vmABI)
	if err != nil {
		fmt.Printf("Error generating Go structs: %v\n", err)
		os.Exit(1)
	}

	// Write the generated code to the output file
	err = ioutil.WriteFile(outputFile, []byte(generatedCode), 0644)
	if err != nil {
		fmt.Printf("Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated Go structs in %s\n", outputFile)
}
