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
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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

var apiRequestHeader map[string]string
var gitlabAPI string
var payload Payload
var gitlabHost string
var port string
var projectPath string
var protocol = "https"
var err error

func main() {

	cmd := filepath.Base(os.Args[0])
	usage := func() {
		fmt.Fprintf(os.Stderr, "Usage: %s expects input (json payload) from stdin. It servers the %s purpuse of concourse resource type.\n", cmd, cmd)
	}

	scanner := bufio.NewScanner(os.Stdin)
	var b bytes.Buffer
	for scanner.Scan() {
		b.WriteString(scanner.Text())
	}

	err = json.Unmarshal(b.Bytes(), &payload)
	if err != nil {
		usage()
		panicIfErr(err)
	}

	apiRequestHeader = map[string]string{"private-token": payload.Source.PrivateToken}
	configureSslVerification()
	decomposeURI()

	switch cmd {
	case "check":
		check()
	case "in":
		if len(os.Args[1]) == 0 {
			panic("in command needs a destination folder argument")
		}
		in(os.Args[1])
	case "out":
		out()
	default:
		usage()
		panic("unknown command")
	}
}

func out() {

}

func in(destFolder string) {

	if len(payload.Source.PrivateKey) != 0 {
		rsaDir := os.ExpandEnv("$HOME/.ssh/ki")
		panicIfErr(os.MkdirAll(rsaDir, os.ModeDir|0644))
		panicIfErr(ioutil.WriteFile(rsaDir+"/id_rsa", []byte(payload.Source.PrivateKey), 0500))

		pars := []string{"-t", "rsa", gitlabHost}
		if len(port) != 0 {
			pars = []string{"-t", "rsa", "-p", port, gitlabHost}
		}

		knownhost, err := exec.Command("ssh-keyscan", pars...).Output()
		panicIfErr(err)

		panicIfErr(ioutil.WriteFile(rsaDir+"/known_hosts", knownhost, 0500))
	} else {
		defLogin := fmt.Sprintf("default login %s password %s", payload.Source.Username, payload.Source.Password)
		panicIfErr(ioutil.WriteFile(os.ExpandEnv("$HOME/.netrc"), []byte(defLogin), 0500))
	}
}

func check() {

	latestVersion := payload.Version["sha"]

	_ = port

	gitlabAPI = fmt.Sprintf("%s://%s/api/v4/projects/%s", protocol, gitlabHost, url.PathEscape(projectPath))

	if payload.Source.NoSsl {
		protocol = "http"
	}

	lastProcessedMrCommitTs := time.Time{}

	if len(latestVersion) != 0 {
		lastProcessedMrCommitTs = getMRLastUpdate(latestVersion)
	}
	newMRs := []*MergeRequest{}
	var openMRs []*MergeRequest
	resp := sendrequest("GET", fmt.Sprintf("%s/merge_requests?state=opened&order_by=updated_at", gitlabAPI), "")
	panicIfErr(json.Unmarshal(resp, &openMRs))

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
	panicIfErr(err)

	fmt.Println(string(result))
}

func getMRLastUpdate(latestVersion string) time.Time {
	resp := sendrequest("GET", fmt.Sprintf("%s/repository/commits/%s", gitlabAPI, latestVersion), "")

	var commit Commit
	panicIfErr(json.Unmarshal(resp, &commit))

	parsed, err := time.Parse(DateTimeLayout, commit.CommittedDate)
	panicIfErr(err)

	return parsed.UTC()
}

func sendrequest(method, url, body string) []byte {

	req, err := http.NewRequest(method, url, bytes.NewBuffer([]byte(body)))
	for k, v := range apiRequestHeader {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	panicIfErr(err)
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result, err := ioutil.ReadAll(resp.Body)
		panicIfErr(err)

		return result
	}

	panic(fmt.Sprintf("request from %s returned status code: %d(%s)", url, resp.StatusCode, resp.Status))
}

func decomposeURI() {
	uri := strings.TrimSpace(payload.Source.URI)
	var re *regexp.Regexp
	if strings.Contains(uri, "git@") {
		re = regexp.MustCompile(".*git@(.*):([0-9]*\\/+)?(.*)\\.git")
		res := re.FindStringSubmatch(uri)
		gitlabHost = res[1]
		port = strings.Trim(res[2], "/")
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
}

func configureSslVerification() {
	if payload.Source.SkipSslVerification {
		os.Setenv("GIT_SSL_NO_VERIFY", "true")
		panicIfErr(ioutil.WriteFile(os.ExpandEnv("HOME/.curlrc"), []byte("insecure"), 0644))
	}
}

func panicIfErr(err error) {
	if err != nil {
		fpcs := make([]uintptr, 1)
		caller := runtime.FuncForPC(fpcs[0] - 1)
		callerMethod, line := caller.FileLine(fpcs[0] - 1)

		fmt.Printf("\nPanic at %s : %d \n", callerMethod, line)

		panic(err)
	}
}
