package main

import (
	"bytes"
	"fmt"
	"log"
	"strconv"
)

func LsTree(sha1Hash string, nameOnly bool) {
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

		if offset+SHA1_HASH_LENGTH > len(data) {
			return nil, fmt.Errorf("SHA-1 hash is missing or incomplete")
		}
		sha1Hash := fmt.Sprintf("%x", data[offset:offset+SHA1_HASH_LENGTH])

		entryContent, err := readContentFromSha(sha1Hash)
		if err != nil {
			return nil, err
		}

		objectTypeRaw := bytes.SplitN(entryContent, []byte(" "), 2)[0]
		objectType, err := stringToBlobType(string(objectTypeRaw))
		if err != nil {
			return nil, err
		}

		treeEntries = append(treeEntries, TreeEntry{
			mode:     Mode(mode),
			object:   objectType,
			sha1Hash: []byte(sha1Hash),
			name:     fileName,
		})

		offset += SHA1_HASH_LENGTH
	}

	return treeEntries, nil
}
