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
	"strconv"
)

const (
	SHA1HashLength = 20
)

type Mode int

const (
	DIR             Mode = 40000
	REGULAR_FILE    Mode = 100644
	EXECUTABLE_FILE Mode = 100755
	SYMBOLIC_LINK   Mode = 120000
)

type ObjectType string

const (
	TREE ObjectType = "tree"
	BLOB ObjectType = "blob"
)

func stringToObjectType(str string) (ObjectType, error) {
	switch str {
	case "tree":
		return TREE, nil
	case "blob":
		return BLOB, nil
	default:
		return "", fmt.Errorf("invalid ObjectType: %s", str)
	}
}

type TreeEntry struct {
	mode     Mode
	object   ObjectType
	sha1Hash string
	name     string
}

func readContentFromSha(sha1Hash string) ([]byte, error) {
	filepath := path.Join(".git", "objects", sha1Hash[:2], sha1Hash[2:])
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("unable to open file: %v", err)
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("unable to read file content: %v", err)
	}

	reader, err := zlib.NewReader(bytes.NewReader(fileContent))
	if err != nil {
		return nil, fmt.Errorf("unable to create zlib reader: %v", err)
	}
	defer reader.Close()

	decompressedContent, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to decompress content: %v", err)
	}

	return decompressedContent, nil
}

func parseTreeEntries(data []byte) ([]TreeEntry, error) {
	treeEntries := []TreeEntry{}
	offset := 0
	for offset < len(data) {
		modeEndOffset := bytes.IndexByte(data[offset:], ' ')
		if modeEndOffset == -1 {
			return nil, fmt.Errorf("mode not found in data")
		}

		modeStr := string(data[offset : offset+modeEndOffset])
		mode, err := strconv.Atoi(modeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid mode: %v", err)
		}

		offset += modeEndOffset + 1

		nameEndOffset := bytes.IndexByte(data[offset:], 0)
		if nameEndOffset == -1 {
			return nil, fmt.Errorf("name not found in data")
		}

		fileName := string(data[offset : offset+nameEndOffset])
		offset += nameEndOffset + 1

		if offset+SHA1HashLength > len(data) {
			return nil, fmt.Errorf("SHA-1 hash is missing or incomplete")
		}
		sha1Hash := fmt.Sprintf("%x", data[offset:offset+SHA1HashLength])

		entryContent, err := readContentFromSha(sha1Hash)
		if err != nil {
			return nil, err
		}

		objectTypeRaw := bytes.SplitN(entryContent, []byte(" "), 2)[0]
		objectType, err := stringToObjectType(string(objectTypeRaw))
		if err != nil {
			return nil, err
		}

		treeEntries = append(treeEntries, TreeEntry{
			mode:     Mode(mode),
			object:   objectType,
			sha1Hash: sha1Hash,
			name:     fileName,
		})

		offset += SHA1HashLength
	}

	return treeEntries, nil
}

func computeHashAndStoreObject(headerBytes []byte, fileContent []byte) (string, error) {
	objectContent := append(headerBytes, 0)
	objectContent = append(objectContent, fileContent...)

	h := sha1.New()
	h.Write(objectContent)
	shaRaw := h.Sum(nil)
	sha1Hash := fmt.Sprintf("%x", shaRaw)

	dir := path.Join(".git", "objects", sha1Hash[:2])
	outputPath := path.Join(dir, sha1Hash[2:])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create object directory: %v", err)
	}

	// Compress and write the object to file
	var buf bytes.Buffer
	compressor := zlib.NewWriter(&buf)
	_, err := compressor.Write(objectContent)
	if err != nil {
		return "", fmt.Errorf("failed to write to zlib: %v", err)
	}
	compressor.Close()

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %v", err)
	}
	defer outputFile.Close()

	_, err = outputFile.Write(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("failed to write compressed content to file: %v", err)
	}

	return sha1Hash, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: ./your_program.sh <command> [<args>...]\n")
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
		if len(os.Args) < 4 {
			log.Fatal("Missing arguments for cat-file")
		}
		sha1Hash := os.Args[3]
		data, err := readContentFromSha(sha1Hash)
		if err != nil {
			log.Fatal(err)
		}
		results := bytes.SplitN(data, []byte{0}, 2)
		fmt.Print(string(results[1]))

	case "hash-object":
		if len(os.Args) < 4 {
			log.Fatal("Missing file argument for hash-object")
		}
		filepath := os.Args[3]
		fileContent, err := os.ReadFile(filepath)
		if err != nil {
			log.Fatalf("Unable to read file %s: %v\n", filepath, err)
		}

		fileSize := len(fileContent)
		header := fmt.Sprintf("blob %d", fileSize)
		headerBytes := []byte(header)

		sha1Hash, err := computeHashAndStoreObject(headerBytes, fileContent)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%s\n", sha1Hash)

	case "ls-tree":

		sha1Hash := os.Args[2]
		nameOnly := false

		if os.Args[2] == "--name-only" {
			sha1Hash = os.Args[3]
			nameOnly = true
		}

		data, err := readContentFromSha(sha1Hash)
		if err != nil {
			log.Fatal(err)
		}

		content := bytes.SplitN(data, []byte{0}, 2)[1]
		treeEntries, err := parseTreeEntries(content)
		if err != nil {
			log.Fatal(err)
		}

		for _, entry := range treeEntries {
			if nameOnly {
				fmt.Printf("%s\n", entry.name)
			} else {
				fmt.Printf("%06d %s %x  %s\n", entry.mode, entry.object, entry.sha1Hash, entry.name)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
