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
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

const (
	SHA1HashLength = 20
	GIT_DIR        = ".git"
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

type TreeEntry struct {
	mode     Mode
	object   ObjectType
	sha1Hash []byte
	name     string
}

type Tree struct {
	entries []TreeEntry
}

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

func readContentFromSha(sha1Hash string) ([]byte, error) {
	filepath := path.Join(GIT_DIR, "objects", sha1Hash[:2], sha1Hash[2:])
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
			sha1Hash: []byte(sha1Hash),
			name:     fileName,
		})

		offset += SHA1HashLength
	}

	return treeEntries, nil
}

func computeHashAndStoreObject(content []byte) ([]byte, error) {
	h := sha1.New()
	h.Write(content)
	shaRaw := h.Sum(nil)
	sha1Hash := fmt.Sprintf("%x", shaRaw)

	dir := path.Join(GIT_DIR, "objects", sha1Hash[:2])
	outputPath := path.Join(dir, sha1Hash[2:])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %v", err)
	}

	// Compress and write the object to file
	var buf bytes.Buffer
	compressor := zlib.NewWriter(&buf)
	_, err := compressor.Write(content)
	if err != nil {
		return nil, fmt.Errorf("failed to write to zlib: %v", err)
	}
	compressor.Close()

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %v", err)
	}
	defer outputFile.Close()

	_, err = outputFile.Write(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to write compressed content to file: %v", err)
	}

	return shaRaw, nil
}

func hashObject(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Not able to read file %s: %v", filename, err)
	}
	defer file.Close()

	contents, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("Not able to read file %s: %v", filename, err)
	}

	blob := fmt.Sprintf("%s %d%c%s", "blob", len(contents), 0, contents)

	shaCodeRaw, _ := computeHashAndStoreObject([]byte(blob))
	return shaCodeRaw, nil
}

func generateTreeFromDir(dirname string) (Tree, error) {
	var tree Tree

	files, err := os.ReadDir(dirname)
	if err != nil {
		return tree, err
	}

	for _, file := range files {
		filePath := filepath.Join(dirname, file.Name()) // full path of the file
		if file.IsDir() {
			// Skip the .git directory
			if file.Name() == ".git" {
				continue
			}

			newTree, err := generateTreeFromDir(filePath)
			if err != nil {
				return tree, err
			}
			treeSha, err := storeTreeObject(newTree)
			if err != nil {
				return tree, err
			}
			tree.entries = append(tree.entries, TreeEntry{
				mode:     DIR,
				name:     file.Name(),
				sha1Hash: treeSha,
				object:   TREE,
			})
		} else {
			shaCode, _ := hashObject(filePath)
			tree.entries = append(tree.entries, TreeEntry{
				mode:     REGULAR_FILE,
				name:     file.Name(),
				sha1Hash: shaCode,
				object:   BLOB,
			})
		}
	}

	return tree, nil
}

func storeTreeObject(tree Tree) ([]byte, error) {
	var treeData []byte
	entries := tree.entries
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	for _, entry := range entries {
		treeData = append(treeData, []byte(fmt.Sprintf("%d %s\x00", entry.mode, entry.name))...)
		treeData = append(treeData, entry.sha1Hash...)
	}

	treeSize := len(treeData)
	header := []byte(fmt.Sprintf("tree %d\x00", treeSize))

	var treeBlob []byte
	treeBlob = append(treeBlob, header...)
	treeBlob = append(treeBlob, treeData...)

	rawSha, err := computeHashAndStoreObject(treeBlob)
	if err != nil {
		return nil, err
	}
	return rawSha, nil
}

func generateCommitObject(treeSha1Hash string, message string, username string, email string, parent *string) []byte {
	timestampNow := time.Now().Unix()
	_, timeZoneOffset := time.Now().Zone()
	timeZoneOffsetStr := fmt.Sprintf("%+03d%02d", timeZoneOffset/3600, (timeZoneOffset%3600)/60)

	commitContent := []byte("tree " + treeSha1Hash + "\n")

	if parent != nil {
		commitContent = append(commitContent, []byte(fmt.Sprintf("parent %s\n", *parent))...)
	}

	commitContent = append(commitContent, []byte(fmt.Sprintf("author %s <%s> %d %s\n", username, email, timestampNow, timeZoneOffsetStr))...)
	commitContent = append(commitContent, []byte(fmt.Sprintf("committer %s <%s> %d %s\n\n", username, email, timestampNow, timeZoneOffsetStr))...)
	commitContent = append(commitContent, []byte(fmt.Sprintf("%s\n", message))...)

	commitObject := []byte(fmt.Sprintf("commit %d\x00", len(commitContent)))
	commitObject = append(commitObject, commitContent...)
	return commitObject
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: ./your_program.sh <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		for _, dir := range []string{GIT_DIR, path.Join(GIT_DIR, "objects"), path.Join(GIT_DIR, "refs")} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			}
		}

		headFileContents := []byte("ref: refs/heads/main\n")
		if err := os.WriteFile(path.Join(GIT_DIR, "HEAD"), headFileContents, 0644); err != nil {
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
		sha1Hash, err := hashObject(filepath)
		if err != nil {
			log.Fatal("Error generating hash-object")
		}
		fmt.Printf("%x\n", sha1Hash)

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

	case "write-tree":
		tree, err := generateTreeFromDir(".")
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		sha, err := storeTreeObject(tree)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		fmt.Printf("%x\n", sha)
	case "commit-tree":
		var message string
		var parent string
		var username string = "Krishna Agrawal"
		var email string = "imkrishnaagrawal@gmail.com"

		if os.Args[3] == "-m" {
			message = os.Args[4]
		} else {
			message = os.Args[6]
		}

		if os.Args[3] == "-p" {
			parent = os.Args[4]

		}

		treeSha1Hash := os.Args[2]
		commitObject := generateCommitObject(treeSha1Hash, message, username, email, &parent)
		rawSha, _ := computeHashAndStoreObject(commitObject)
		fmt.Printf("%x", rawSha)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
