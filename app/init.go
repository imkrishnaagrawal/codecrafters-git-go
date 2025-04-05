package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func Init(basePath string) {
	for _, dir := range []string{basePath, filepath.Join(basePath, "objects"), filepath.Join(basePath, "refs")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(filepath.Join(basePath, "HEAD"), headFileContents, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
	}
	fmt.Println("Initialized git directory")
}
