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

type Payload struct {
	Source  Psource           `json:"source"`
	Version map[string]string `json:"version"`
	Params  Params            `json:"params"`
}

type Psource struct {
	URI                 string `json:"uri"`
	PrivateToken        string `json:"private_token"`
	PrivateKey          string `json:"private_key"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	NoSsl               bool   `json:"no_ssl"`
	SkipSslVerification bool   `json:"skip_ssl_verification"`
	ConcourseHost       string `json:"concourse_host"`
}

type Commit struct {
	CommittedDate string `json:"committed_date"`
}

type MergeRequest struct {
	Sha string `json:"sha"`
}

type Params struct {
	Repository string `json:"repository"`
	Status     string `json:"status"`
	BuildLabel string `json:"build_label"`
}

const defaultBuildLabel = "Concourse"

var dateLayouts = [...]string{"2006-01-02T15:04:05.000-07:00", "2006-01-02T15:04:05.000Z"}

var gitlabAPIbase string
var payload Payload
var gitlabHost string
var port string
var projectPath string
var protocol string
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

	checkRequired()
	configureSslVerification()
	decomposeURI()

	if payload.Source.NoSsl {
		protocol = "http"
	} else {
		protocol = "https"
	}
	gitlabAPIbase = fmt.Sprintf("%s://%s/api/v4/projects/%s/", protocol, gitlabHost, url.PathEscape(projectPath))

	var result interface{}

	switch cmd {
	case "check":
		result = check()
	case "in":
		if len(os.Args[1]) == 0 {
			panic("in command needs a destination folder argument")
		}
		result = in(os.Args[1])
	case "out":
		if len(os.Args[1]) == 0 {
			panic("out command needs a source folder argument")
		}
		result = out(os.Args[1])
	default:

		panic("unknown command")
	}

	output, err := json.Marshal(result)
	panicIfErr(err)

	fmt.Println(string(output))
}

func checkRequired() {
	s := payload.Source
	required := []string{s.PrivateToken, s.URI, s.PrivateKey, s.ConcourseHost}
	for _, val := range required {
		if len(val) == 0 {
			panic(fmt.Sprintf("please specify all the required parameters"))
		}
	}
}

func out(sourceFolder string) map[string]map[string]string {

	if len(payload.Params.Repository) == 0 {
		panic("please specify a repository")
	}
	if len(payload.Params.Status) == 0 {
		panic("please specify a status")
	}
	if len(payload.Source.ConcourseHost) == 0 {
		panic("please specify the concourse host address. (format url:port)")
	}
	if len(payload.Params.BuildLabel) == 0 {
		payload.Params.BuildLabel = defaultBuildLabel
	}

	panicIfErr(os.Chdir(sourceFolder))
	panicIfErr(os.Chdir(payload.Params.Repository))

	targetURL := fmt.Sprintf("%s/teams/%s/pipelines/%s/jobs/%s/builds/%s",
		payload.Source.ConcourseHost,
		url.PathEscape(os.Getenv("BUILD_TEAM_NAME")),
		url.PathEscape(os.Getenv("BUILD_PIPELINE_NAME")),
		url.PathEscape(os.Getenv("BUILD_JOB_NAME")),
		url.PathEscape(os.Getenv("BUILD_NAME")))

	out, err := exec.Command("git", "log", "--skip=1", "--format=%H", "-n", "1").Output()
	panicIfErr(err)
	commitSHA := strings.Trim(string(out), "\n\" ")

	bodyJSON, err := json.Marshal(map[string]interface{}{
		"name":       payload.Params.BuildLabel,
		"state":      payload.Params.Status,
		"target_url": targetURL,
	})
	panicIfErr(err)

	header := map[string]string{
		"Content-Type": "application/json",
	}

	sendAPIRequest("POST", "statuses/"+commitSHA, bodyJSON, header)

	return map[string]map[string]string{
		"version": map[string]string{
			"sha": fmt.Sprintf("%s", commitSHA),
		},
	}
}

func in(destFolder string) map[string]map[string]string {

	if len(payload.Source.PrivateKey) != 0 {
		rsaDir := os.ExpandEnv("$HOME/.ssh/")
		panicIfErr(os.MkdirAll(rsaDir, os.ModeDir|0744))
		panicIfErr(ioutil.WriteFile(rsaDir+"id_rsa", []byte(payload.Source.PrivateKey), 0500))

		pars := []string{"-t", "rsa", gitlabHost}
		if len(port) != 0 {
			pars = []string{"-t", "rsa", "-p", port, gitlabHost}
		}

		knownhost, err := exec.Command("ssh-keyscan", pars...).Output()
		panicIfErr(err)

		panicIfErr(ioutil.WriteFile(rsaDir+"/known_hosts", knownhost, 0500))
	} else {
		defLogin := fmt.Sprintf("default login %s password %s", payload.Source.Username, payload.Source.Password)
		panicIfErr(ioutil.WriteFile(os.ExpandEnv("$HOME/.netrc"), []byte(defLogin), 0644))
	}

	panicIfErrMsg(exec.Command("git", "clone", payload.Source.URI, destFolder).Run(), "Cannot clone the repository")
	panicIfErrMsg(os.Chdir(destFolder), "Cannot go to destination folder")

	gitmerge := exec.Command("git", "merge", "-m", "local merge", payload.Version["sha"])
	stderr, err := gitmerge.StderrPipe()
	panicIfErr(err)
	panicIfErr(gitmerge.Start())
	slurp, _ := ioutil.ReadAll(stderr)
	if err = gitmerge.Wait(); err != nil {
		panicIfErr(fmt.Errorf("merge error: %s/n", string(slurp)))
	}

	return map[string]map[string]string{"version": payload.Version}
}

func check() []*MergeRequest {

	latestVersion := payload.Version["sha"]
	lastProcessedMrCommitTs := time.Time{}
	if len(latestVersion) != 0 {
		lastProcessedMrCommitTs = getMRLastCommitTimestamp(latestVersion)
	}

	var newMRs *MergeRequest
	var openMRs []*MergeRequest
	resp := sendAPIRequest("GET", "merge_requests?state=opened&order_by=updated_at", nil, nil)
	panicIfErr(json.Unmarshal(resp, &openMRs))

	var oldestNotProcessedTS *time.Time
	for _, mr := range openMRs {
		if len(mr.Sha) > 0 {
			lastCommitTs := getMRLastCommitTimestamp(mr.Sha)
			if lastCommitTs.After(lastProcessedMrCommitTs) && (oldestNotProcessedTS == nil || lastCommitTs.Before(*oldestNotProcessedTS)) {
				oldestNotProcessedTS = &lastCommitTs
				newMRs = mr
			}
		}
	}
	if newMRs == nil {
		newMRs = &MergeRequest{Sha: latestVersion}
	}

	return []*MergeRequest{newMRs}
}

func getMRLastCommitTimestamp(latestVersion string) time.Time {
	resp := sendAPIRequest("GET", fmt.Sprintf("repository/commits/%s", latestVersion), nil, nil)

	var commit Commit
	panicIfErrMsg(json.Unmarshal(resp, &commit), "Unable to unmarshal merge request the response")

	// try parsing the time to the expected formats
	parsed, err := time.Parse(dateLayouts[0], commit.CommittedDate)
	for i := 1; err != nil && i < len(dateLayouts); i++ {
		parsed, err = time.Parse(dateLayouts[i], commit.CommittedDate)
	}
	panicIfErrMsg(err, "Unable to parse time string")

	return parsed.UTC()
}

func sendAPIRequest(method, suburl string, body []byte, header map[string]string) []byte {

	url := gitlabAPIbase + suburl

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))

	req.Header.Set("private-token", payload.Source.PrivateToken)
	for k, v := range header {
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

	panicIfErrMsg(fmt.Errorf(""), fmt.Sprintf("request sent to '%s' returned with non-success status: %s", url, resp.Status))
	return nil
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
		fmt.Fprintf(os.Stderr, "\npanic at %s:\n", getCallerInfo())
		panic(err)
	}
}

func panicIfErrMsg(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %s\npanic at %s:\n", msg, getCallerInfo())
		panic(err)
	}
}

func getCallerInfo() string {
	fpcs := make([]uintptr, 1)
	runtime.Callers(3, fpcs)
	caller := runtime.FuncForPC(fpcs[0] - 1)
	file, line := caller.FileLine(fpcs[0] - 1)
	return fmt.Sprintf("%s(%d)", file, line)
}
