package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"log"
	"os"
	"path"
)

// Usage: your_program.sh <command> <arg1> <arg2> ...
func main() {

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":

		for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			}
		}

		headFileContents := []byte("ref: refs/heads/main\n")
		if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}

		fmt.Println("Initialized git directory")
	case "cat-file":
		sha := os.Args[3]
		filepath := path.Join(".git", "objects", sha[:2], sha[2:])
		file, err := os.Open(filepath)
		if err != nil {
			log.Fatal("Unable to read file")
		}
		defer file.Close()

		fileContent, err := io.ReadAll(file)
		if err != nil {
			log.Fatal(err)
		}

		reader, err := zlib.NewReader(bytes.NewReader(fileContent))
		if err != nil {
			log.Fatal(err)
		}
		defer reader.Close()

		decompressedContent, err := io.ReadAll(reader)
		if err != nil {
			log.Fatal(err)
		}

		results := bytes.SplitN(decompressedContent, []byte{0}, 2)

		fmt.Print(string(results[1]))

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
