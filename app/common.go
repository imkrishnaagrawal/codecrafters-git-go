package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func CatFile(sha1Hash string) {

	data, err := readContentFromSha(sha1Hash)
	if err != nil {
		log.Fatal(err)
	}
	results := bytes.SplitN(data, []byte{0}, 2)
	fmt.Print(string(results[1]))
}
func readContentFromSha(sha1Hash string) ([]byte, error) {
	filepath := filepath.Join(GIT_DIR, "objects", sha1Hash[:2], sha1Hash[2:])
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

func computeHashAndStoreObject(basePath string, content []byte) ([]byte, error) {
	h := sha1.New()
	h.Write(content)
	shaRaw := h.Sum(nil)
	sha1Hash := fmt.Sprintf("%x", shaRaw)

	dir := filepath.Join(basePath, "objects", sha1Hash[:2])
	outputPath := filepath.Join(dir, sha1Hash[2:])
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
