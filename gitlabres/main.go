package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const DateTimeLayout = "2006-01-02T15:04:05.000-07:00"

type Payload struct {
	Source  Psource           `json:"source"`
	Version map[string]string `json:"version"`
	Params  map[string]string `json:"params"`
}

type Psource struct {
	URI                 string `json:"uri"`
	PrivateToken        string `json:"private_token"`
	PrivateKey          string `json:"private_key"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	NoSsl               bool   `json:"no_ssl"`
	SkipSslVerification bool   `json:"skip_ssl_verification"`
}

type Commit struct {
	CommittedDate string `json:"committed_date"`
}

type MergeRequest struct {
	Sha string `json:"sha"`
}

var requestHeader map[string]string
var gitlabAPI string
var payload []byte
var err error

func main() {

	scanner := bufio.NewScanner(os.Stdin)
	var b bytes.Buffer

	for scanner.Scan() {
		b.WriteString(scanner.Text())
	}

	payload = b.Bytes()
	if len(payload) == 0 {
		panic("no payload is passed")
	}

	cmd := filepath.Base(os.Args[0])

	usage := func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -cmd <Command Name> -payloadF <Payload file name>\n", filepath.Base(os.Args[0]))
	}

	switch cmd {
	case "check":
		check()
	default:
		usage()
		panic("unknown command")
	}
}

func check() {
	var jsonp Payload
	err := json.Unmarshal(payload, &jsonp)
	if err != nil {
		panic(err)
	}

	requestHeader = map[string]string{"private-token": jsonp.Source.PrivateToken}

	uri := strings.TrimSpace(jsonp.Source.URI)
	gitlabHost := ""
	port := ""
	projectPath := ""
	protocol := "https"
	latestVersion := jsonp.Version["sha"]
	var re *regexp.Regexp
	if strings.Contains(uri, "git@") {
		re = regexp.MustCompile(".*git@(.*):([0-9]*\\/+)?(.*)\\.git")
		res := re.FindStringSubmatch(uri)
		gitlabHost = res[1]
		port = res[2]
		projectPath = res[3]

	} else if strings.Index(uri, "http") == 0 {
		re = regexp.MustCompile("(https?):\\/\\/([^\\/]*)\\/(.*)\\.git")
		res := re.FindStringSubmatch(uri)
		protocol = res[1]
		gitlabHost = res[2]
		projectPath = res[3]
	} else {
		panic(fmt.Sprintf("The url protocol is not supported: %s", uri))
	}

	_ = port

	gitlabAPI = fmt.Sprintf("%s://%s/api/v4/projects/%s", protocol, gitlabHost, url.PathEscape(projectPath))

	if jsonp.Source.NoSsl {
		protocol = "http"
	}

	lastProcessedMrCommitTs := time.Time{}

	if len(latestVersion) != 0 {
		lastProcessedMrCommitTs = getMRLastUpdate(latestVersion)
	}
	newMRs := []*MergeRequest{}
	var openMRs []*MergeRequest
	resp := sendrequest("GET", fmt.Sprintf("%s/merge_requests?state=opened&order_by=updated_at", gitlabAPI), "", requestHeader)
	err = json.Unmarshal(resp, &openMRs)
	if err != nil {
		panic(err)
	}

	for _, mr := range openMRs {
		if len(mr.Sha) > 0 {
			lastCommitTs := getMRLastUpdate(mr.Sha)
			if lastCommitTs.After(lastProcessedMrCommitTs) {
				newMRs = append(newMRs, mr)
			}
		}
	}

	if len(newMRs) == 0 {
		newMRs = append(newMRs, &MergeRequest{Sha: latestVersion})
	}

	result, err := json.Marshal(newMRs)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(result))
}

func getMRLastUpdate(latestVersion string) time.Time {
	resp := sendrequest("GET", fmt.Sprintf("%s/repository/commits/%s", gitlabAPI, latestVersion), "", requestHeader)

	var commit Commit
	err := json.Unmarshal(resp, &commit)
	if err != nil {
		panic(err)
	}

	parsed, err := time.Parse(DateTimeLayout, commit.CommittedDate)
	if err != nil {
		panic(err)
	}

	return parsed.UTC()
}

func sendrequest(method, url, body string, header map[string]string) []byte {

	req, err := http.NewRequest(method, url, bytes.NewBuffer([]byte(body)))
	for k, v := range header {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}

		return result
	}

	panic(fmt.Sprintf("request from %s returned status code: %d(%s)", url, resp.StatusCode, resp.Status))
}
