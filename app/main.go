package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"os"
	"path"
)

func readContentFromSha(sha string) ([]byte, error) {
	filepath := path.Join(".git", "objects", sha[:2], sha[2:])
	file, err := os.Open(filepath)
	if err != nil {
		return []byte{0}, err
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return []byte{0}, err
	}

	reader, err := zlib.NewReader(bytes.NewReader(fileContent))
	if err != nil {
		return []byte{0}, err
	}
	defer reader.Close()

	decompressedContent, err := io.ReadAll(reader)
	if err != nil {
		log.Fatal(err)
	}

	return decompressedContent, nil
}

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
		data, err := readContentFromSha(sha)
		if err != nil {
			log.Fatal(err)
		}
		results := bytes.SplitN(data, []byte{0}, 2)
		fmt.Print(string(results[1]))

	case "hash-object":
		filepath := os.Args[3]

		file, err := os.Open(filepath)
		if err != nil {
			log.Fatal("Unable to read file")
		}
		defer file.Close()

		fileContent, err := io.ReadAll(file)
		if err != nil {
			log.Fatal(err)
		}

		fileSize := len(fileContent)

		// Create the header in the format "blob <size> \0"
		header := fmt.Sprintf("blob %d", fileSize)

		// Convert the header to a byte slice
		headerBytes := []byte(header)

		finalBlob := append(headerBytes, 0)
		// Combine the header and file content to form the final blob
		finalBlob = append(finalBlob, fileContent...)

		h := sha1.New()
		h.Write(finalBlob)
		shaRaw := h.Sum(nil)
		shaCode := fmt.Sprintf("%x", shaRaw)

		dir := path.Join(".git", "objects", shaCode[:2])
		outfilePath := path.Join(dir, shaCode[2:])

		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatal("failed to write to objects dir")
		}

		var b bytes.Buffer
		w := zlib.NewWriter(&b)
		w.Write(finalBlob)
		w.Close()

		outputFile, err := os.Create(outfilePath)
		if err != nil {
			log.Fatal(err)
		}
		defer outputFile.Close()

		// Write the compressed content to the file
		_, err = outputFile.Write(b.Bytes())
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%s\n", shaCode)

	case "ls-tree":
		sha := os.Args[3]

		data, err := readContentFromSha(sha)
		if err != nil {
			log.Fatal(err)
		}

		tree := bytes.Split(data, []byte{0})

		for i := 1; i < len(tree)-1; i++ {
			fmt.Printf("%s\n", bytes.Split(tree[i], []byte{32})[1])
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}

// <mode> <name>\0<20_byte_sha><mode> <name>\0<20_byte_sha>
