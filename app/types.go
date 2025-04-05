package main

import "fmt"

type Mode int
type ObjectType int
type BlobType string

const (
	INVALID       ObjectType = 0
	OBJ_COMMIT    ObjectType = 1
	OBJ_TREE      ObjectType = 2
	OBJ_BLOB      ObjectType = 3
	OBJ_TAG       ObjectType = 4
	OBJ_OFS_DELTA ObjectType = 6
	OBJ_REF_DELTA ObjectType = 7
)

const (
	DIR             Mode = 40000
	REGULAR_FILE    Mode = 100644
	EXECUTABLE_FILE Mode = 100755
	SYMBOLIC_LINK   Mode = 120000
)

const (
	TREE BlobType = "tree"
	BLOB BlobType = "blob"
)

type TreeEntry struct {
	mode     Mode
	object   BlobType
	sha1Hash []byte
	name     string
}

type Tree struct {
	entries []TreeEntry
}

func ObjectTypeName(t ObjectType) string {
	switch t {

	case OBJ_COMMIT:
		return "OBJ_COMMIT"
	case OBJ_TREE:
		return "OBJ_TREE"
	case OBJ_BLOB:
		return "OBJ_BLOB"
	case OBJ_TAG:
		return "OBJ_TAG"
	case OBJ_OFS_DELTA:
		return "OBJ_OFS_DELTA"
	case OBJ_REF_DELTA:
		return "OBJ_REF_DELTA"
	case INVALID:
		return "INVALID"
	default:
		return "Unknown"
	}
}

func stringToBlobType(str string) (BlobType, error) {
	switch str {
	case "tree":
		return TREE, nil
	case "blob":
		return BLOB, nil
	default:
		return "", fmt.Errorf("invalid BlobType: %s", str)
	}
}
