package main

import (
	"fmt"
	"time"
)

func CommitTree(treeSha1Hash string, message string, username string, email string, parent *string) {
	commitObject := generateCommitObject(treeSha1Hash, message, username, email, parent)
	rawSha, _ := computeHashAndStoreObject(GIT_DIR, commitObject)
	fmt.Printf("%x", rawSha)
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
