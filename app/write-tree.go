package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func WriteTree() {
	tree, err := generateTreeFromDir(GIT_DIR, ".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	sha, err := storeTreeObject(GIT_DIR, tree)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	fmt.Printf("%x\n", sha)
}

func storeTreeObject(basePath string, tree Tree) ([]byte, error) {
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

	rawSha, err := computeHashAndStoreObject(basePath, treeBlob)
	if err != nil {
		return nil, err
	}
	return rawSha, nil
}

func generateTreeFromDir(basePath string, dirname string) (Tree, error) {
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

			newTree, err := generateTreeFromDir(basePath, filePath)
			if err != nil {
				return tree, err
			}
			treeSha, err := storeTreeObject(basePath, newTree)
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
			shaCode := HashObject(basePath, filePath, false)
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
