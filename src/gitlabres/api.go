package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

//decoupling to enable unit-testing
var sendAPIRequestFunc = sendAPIRequest

func sendAPIRequest(method, suburl string, body []byte, header map[string]string) []byte {

	url := pl.gitlabAPIbase + suburl

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))

	req.Header.Set("private-token", pl.Source.PrivateToken)
	for k, v := range header {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	exitIfErr(err)
	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	exitIfErr(err)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return result
	}

	exitIfErrMsg(fmt.Errorf(""), fmt.Sprintf("request sent to '%s' returned %s\n%s", url, resp.Status, string(result)))
	return nil
}

func cloneGitRepository(destFolder string) {
	exitIfErrMsg(exec.Command("git", "clone", pl.Source.URI, destFolder).Run(), "Cannot clone the repository")
	exitIfErrMsg(os.Chdir(destFolder), "Cannot go to destination folder")
}

func mergeGitRepository(destFolder string) {
	mergeResult, err := exec.Command("git", "merge", "-m", "local merge", pl.Version.SHA).Output()
	exitIfErrMsg(err, "Merge error: "+string(mergeResult))
}
