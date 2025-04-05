package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type DeltaObject struct {
	baseObject string
	data       []byte
}

func readUint32BigEndian(bytes []byte) uint32 {
	return uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
}

func readPktLine(blob []byte) (int, []byte, error) {
	pktLength := blob[:4]
	blob = blob[4:]
	dst := [2]byte{}
	_, err := hex.Decode(dst[:], pktLength)
	if err != nil {
		return 0, nil, err
	}

	size := uint16(dst[0])<<8 | uint16(dst[1])
	if size == 0 {
		return 4, []byte{}, nil
	}
	if len(blob) < int(size)-4 {
		return 4, nil, errors.New("Error reading pkt line")
	}

	data := blob[:size-4]
	if data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	return int(size), data, nil
}

func getObjectName(pktLines [][]byte) (string, error) {
	for _, pktLine := range pktLines[1:] {
		if len(pktLine) == 0 {
			continue
		}
		var hash, ref string
		fmt.Sscanf(string(pktLine), "%s %s", &hash, &ref)
		if ref == "refs/heads/main" || ref == "refs/heads/master" {
			return hash, nil
		}
	}
	return "", errors.New("Invalid pktLines")
}

func getPackfile(cloneUrl string) ([]byte, string, error) {
	response, err := http.Get(fmt.Sprintf("%s/info/refs?service=git-upload-pack", cloneUrl))
	if err != nil {
		return nil, "", err
	}

	discoveryBuffer := bytes.Buffer{}
	io.Copy(&discoveryBuffer, response.Body)
	discovery := discoveryBuffer.Bytes()
	pktLines := [][]byte{}

	for len(discovery) > 0 {
		n, data, err := readPktLine(discovery)
		if err != nil {
			return nil, "", err
		}
		discovery = discovery[n:]
		pktLines = append(pktLines, data)
	}

	objectName, err := getObjectName(pktLines)
	if err != nil {
		return nil, "", err
	}

	buffer := bytes.NewBufferString(fmt.Sprintf("0032want %s\n00000009done\n", objectName))
	response, err = http.Post(fmt.Sprintf("%s/git-upload-pack", cloneUrl), "application/x-git-upload-pack-request", buffer)
	if err != nil {
		return nil, "", err
	}

	packfileBuffer := bytes.Buffer{}
	io.Copy(&packfileBuffer, response.Body)
	packfile := packfileBuffer.Bytes()

	n, _, err := readPktLine(packfile) // read 0008NAK
	if err != nil {
		return nil, "", err
	}
	packfile = packfile[n:]

	return packfile, objectName, nil
}

func writeObjectWithType(object []byte, objectType string) ([]byte, error) {
	blob := bytes.Buffer{}
	fmt.Fprintf(&blob, "%s %d", objectType, len(object))
	blob.WriteByte(0)
	blob.Write(object)
	hash, err := computeHashAndStoreObject(GIT_DIR, blob.Bytes())
	return hash, err
}

func readObjectHeader(packfile []byte) (size uint64, objectType ObjectType, used int, err error) {
	data := packfile[used]
	used++
	objectType = ObjectType((data >> 4) & 0x7)
	size = uint64(data & 0xF)
	shift := 4

	for data&0x80 != 0 {
		if len(packfile) <= used || 64 <= shift {
			return 0, ObjectType(0), 0, errors.New("Bad object header")
		}
		data = packfile[used]
		used++
		size += uint64(data&0x7F) << shift
		shift += 7
	}
	return size, objectType, used, nil
}

func readSize(packfile []byte) (size uint64, used int, err error) {
	data := packfile[used]
	used++
	size = uint64(data & 0x7F)
	shift := 7

	for data&0x80 != 0 {
		if len(packfile) <= used || 64 <= shift {
			return 0, 0, errors.New("Bad size")
		}
		data = packfile[used]
		used++
		size += uint64(data&0x7F) << shift
		shift += 7
	}
	return size, used, nil
}

func readObject(packfile []byte) (int, []byte, error) {
	b := bytes.NewReader(packfile)
	r, err := zlib.NewReader(b)
	if err != nil {
		return 0, nil, err
	}
	defer r.Close()

	object, err := io.ReadAll(r)
	if err != nil {
		return 0, nil, err
	}
	return int(b.Size()) - b.Len(), object, nil
}

func verifyPackfile(packfile []byte) error {
	if len(packfile) < 32 {
		return errors.New("Bad packfile")
	}

	checksum := packfile[len(packfile)-20:]
	packfile = packfile[:len(packfile)-20]
	expected := sha1.Sum(packfile)

	if !bytes.Equal(checksum, expected[:]) {
		return errors.New("Invalid packfile checksum")
	}
	if !bytes.Equal(packfile[0:4], []byte("PACK")) {
		return errors.New("Invalid packfile header")
	}

	version := readUint32BigEndian(packfile[4:8])
	if version != 2 && version != 3 {
		return errors.New("Invalid packfile version")
	}
	return nil
}

func writePackfile(packfile []byte, dir string) error {
	err := verifyPackfile(packfile)
	if err != nil {
		return err
	}

	used := 8
	numObjects := readUint32BigEndian(packfile[used:])
	used += 4
	deltaObjects := []DeltaObject{}
	var objectsRead uint32
	packfile = packfile[:len(packfile)-20]

	for used < len(packfile) {
		objectsRead++
		size, objectType, read, err := readObjectHeader(packfile[used:])
		used += read
		if err != nil {
			return err
		}

		switch objectType {
		case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB, OBJ_TAG:
			read, object, err := readObject(packfile[used:])
			used += read
			if err != nil {
				return err
			}
			if int(size) != len(object) {
				return errors.New("Bad object header length")
			}

			objectTypeStr := map[ObjectType]string{
				OBJ_COMMIT: "commit",
				OBJ_TREE:   "tree",
				OBJ_BLOB:   "blob",
				OBJ_TAG:    "tag",
			}[objectType]

			_, err = writeObjectWithType(object, objectTypeStr)
			if err != nil {
				return err
			}

		case OBJ_OFS_DELTA:
			_, read, err := readSize(packfile[used:])
			used += read
			if err != nil {
				return err
			}
			read, object, err := readObject(packfile[used:])
			used += read
			if err != nil {
				return err
			}
			if int(size) != len(object) {
				return errors.New("Bad object header length")
			}
			return errors.New("can't handle ofsdelta object")

		case OBJ_REF_DELTA:
			hash := packfile[used : used+20]
			used += 20
			read, object, err := readObject(packfile[used:])
			used += read
			if err != nil {
				return err
			}
			if int(size) != len(object) {
				return errors.New("Bad object header length")
			}
			deltaObjects = append(deltaObjects, DeltaObject{
				baseObject: hex.EncodeToString(hash),
				data:       object,
			})

		default:
			return errors.New("Invalid object type")
		}
	}

	if numObjects != objectsRead {
		return errors.New("Bad object count")
	}

	for len(deltaObjects) > 0 {
		unaddedDeltaObjects := []DeltaObject{}
		added := false

		for _, delta := range deltaObjects {
			if objectExists(delta.baseObject) {
				added = true
				baseObject, objectType, err := openObject(delta.baseObject)
				if err != nil {
					return err
				}
				err = writeDeltaObject(baseObject, delta.data, objectType)
				if err != nil {
					return err
				}
			} else {
				unaddedDeltaObjects = append(unaddedDeltaObjects, delta)
			}
		}

		if !added {
			return errors.New("Bad delta objects")
		}
		deltaObjects = unaddedDeltaObjects
	}
	return nil
}

func writeDeltaObject(baseObject, deltaObject []byte, objectType string) error {
	used := 0
	baseSize, read, err := readSize(deltaObject[used:])
	used += read
	if err != nil {
		return err
	}
	if len(baseObject) != int(baseSize) {
		return errors.New("Bad delta header")
	}

	expectedSize, read, err := readSize(deltaObject[used:])
	used += read
	if err != nil {
		return err
	}

	buffer := bytes.Buffer{}
	for used < len(deltaObject) {
		opcode := deltaObject[used]
		used++

		if opcode&0x80 != 0 {
			var argument uint64
			for bit := 0; bit < 7; bit++ {
				if opcode&(1<<bit) != 0 {
					argument += uint64(deltaObject[used]) << (bit * 8)
					used++
				}
			}
			offset := argument & 0xFFFFFFFF
			size := (argument >> 32) & 0xFFFFFF
			if size == 0 {
				size = 0x10000
			}
			buffer.Write(baseObject[offset : offset+size])
		} else {
			size := int(opcode & 0x7F)
			buffer.Write(deltaObject[used : used+size])
			used += size
		}
	}

	undeltifiedObject := buffer.Bytes()
	if int(expectedSize) != len(undeltifiedObject) {
		return errors.New("Bad delta header")
	}

	_, err = writeObjectWithType(undeltifiedObject, objectType)
	return err
}

func objectExists(hash string) bool {
	path := fmt.Sprintf(".git/objects/%s/%s", hash[:2], hash[2:])
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

func findNull(bytes []byte) int {
	for i, val := range bytes {
		if val == 0 {
			return i
		}
	}
	return len(bytes)
}

func openObject(objectName string) ([]byte, string, error) {
	file, err := os.Open(fmt.Sprintf(".git/objects/%s/%s", objectName[:2], objectName[2:]))
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	reader, err := zlib.NewReader(file)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}

	idx := findNull(data)
	var objectType string
	var size int
	fmt.Sscanf(string(data[:idx]), "%s %d", &objectType, &size)

	if idx+size+1 != len(data) {
		return nil, "", errors.New("Bad object size")
	}
	return data[idx+1:], objectType, nil
}

func checkoutCommit(commitHash string) error {
	commit, objectType, err := openObject(commitHash)
	if err != nil {
		return err
	}
	if objectType != "commit" {
		return errors.New("Object not a commit")
	}

	treeHash := commit[5:45]
	return checkoutTree(string(treeHash), ".")
}

func checkoutTree(treeHash, dir string) error {
	os.MkdirAll(dir, 0755)
	data, err := readContentFromSha(treeHash)
	if err != nil {
		log.Fatal(err)
	}

	content := bytes.SplitN(data, []byte{0}, 2)[1]
	treeEntries, err := parseTreeEntries(content)
	if err != nil {
		return errors.New("Object not a tree")
	}

	for _, entry := range treeEntries {
		hashStr := string(entry.sha1Hash)
		fullPath := fmt.Sprintf("%s/%s", dir, entry.name)

		if entry.mode == DIR {
			err = checkoutTree(hashStr, fullPath)
			if err != nil {
				return err
			}
		} else if entry.mode == EXECUTABLE_FILE || entry.mode == REGULAR_FILE {
			blob, objectType, err := openObject(hashStr)
			if err != nil {
				return err
			}
			if objectType != "blob" {
				return errors.New("Object not a blob")
			}
			os.WriteFile(fullPath, blob, 0644)
		}
	}
	return nil
}

func Clone(cloneUrl, dir string) {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		panic(err)
	}

	err = os.Chdir(dir)
	if err != nil {
		panic(err)
	}

	Init(GIT_DIR)

	packfile, commit, err := getPackfile(cloneUrl)
	if err != nil {
		panic(err)
	}

	err = writePackfile(packfile, dir)
	if err != nil {
		panic(err)
	}

	err = checkoutCommit(commit)
	if err != nil {
		panic(err)
	}
}
