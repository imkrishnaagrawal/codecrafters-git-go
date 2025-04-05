package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

func HashObject(basePath string, filename string, print bool) []byte {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("not able to read file %s: %v", filename, err)
	}
	defer file.Close()

	contents, err := io.ReadAll(file)
	if err != nil {
		log.Fatal("not able to read file %s: %v", filename, err)
	}

	blob := fmt.Sprintf("%s %d%c%s", "blob", len(contents), 0, contents)

	shaCodeRaw, _ := computeHashAndStoreObject(basePath, []byte(blob))

	if err != nil {
		log.Fatal("Error generating hash-object")
	}
	if print {
		fmt.Printf("%x\n", shaCodeRaw)
	}

	return shaCodeRaw

}
