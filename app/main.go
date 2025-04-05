package main

import (
	"fmt"
	"log"
	"os"
)

const (
	SHA1_HASH_LENGTH = 20
	GIT_DIR          = ".git"
	META_DATA_END    = 12
	CHECK_SUM_LENGTH = 20
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: ./your_program.sh <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		Init(GIT_DIR)

	case "cat-file":
		if len(os.Args) < 4 {
			log.Fatal("Missing arguments for cat-file")
		}
		sha1Hash := os.Args[3]
		CatFile(sha1Hash)

	case "hash-object":
		if len(os.Args) < 4 {
			log.Fatal("Missing file argument for hash-object")
		}
		HashObject(GIT_DIR, os.Args[3], true)

	case "ls-tree":
		sha1Hash := os.Args[2]
		nameOnly := false
		if os.Args[2] == "--name-only" {
			sha1Hash = os.Args[3]
			nameOnly = true
		}
		LsTree(sha1Hash, nameOnly)

	case "write-tree":
		WriteTree()

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

		CommitTree(os.Args[2], message, username, email, &parent)

	case "clone":
		cloneUrl := os.Args[2]
		dir := os.Args[3]
		Clone(cloneUrl, dir)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
