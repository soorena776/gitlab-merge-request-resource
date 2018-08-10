package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var pl = &Payload{}

func main() {
	cmd := filepath.Base(os.Args[0])
	var usage = func() string {
		return fmt.Sprintf("Usage: %s expects input (json payload) from stdin. It servers the %s purpuse of concourse resource type.\n", cmd, cmd)
	}

	if err := populatePayload(); err != nil {
		exitIfErrMsg(err, usage())
	}

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
			exitIfErr(fmt.Errorf("out command needs a source folder argument"))
		}
		result = out(os.Args[1])
	default:
		exitIfErrMsg(fmt.Errorf("unknown command"), usage())
	}

	output, err := json.Marshal(result)
	exitIfErr(err)

	//output to stdout
	fmt.Println(string(output))
}

func populatePayload() (err error) {
	scanner := bufio.NewScanner(os.Stdin)
	var b bytes.Buffer
	for scanner.Scan() {
		b.WriteString(scanner.Text())
	}

	err = json.Unmarshal(b.Bytes(), &pl)
	if err != nil {
		return
	}
	err = checkRequired()
	if err != nil {
		return
	}

	err = configureSslVerification()
	if err != nil {
		return
	}
	err = decomposeURI()
	if err != nil {
		return
	}
	if pl.Source.NoSsl {
		pl.protocol = "http"
	} else {
		pl.protocol = "https"
	}

	pl.gitlabAPIbase = fmt.Sprintf("%s://%s/api/v4/projects/%s/", pl.protocol, pl.gitlabHost, url.PathEscape(pl.projectPath))
	return
}

func checkRequired() error {
	s := pl.Source
	required := []string{s.PrivateToken, s.URI, s.PrivateKey, s.ConcourseHost}
	for _, val := range required {
		if len(val) == 0 {
			return fmt.Errorf("please specify all the required parameters")
		}
	}

	return nil
}

func parseTime(timestr string) time.Time {

	var dateLayouts = [...]string{"2006-01-02T15:04:05.000-07:00", "2006-01-02T15:04:05.000Z"}

	// try parsing the finished time to the expected formats
	parsed, err := time.Parse(dateLayouts[0], timestr)
	for i := 1; err != nil && i < len(dateLayouts); i++ {
		parsed, err = time.Parse(dateLayouts[i], timestr)
	}
	exitIfErrMsg(err, "Unable to parse time string")

	return parsed.UTC()
}

func decomposeURI() (err error) {
	uri := strings.TrimSpace(pl.Source.URI)
	var re *regexp.Regexp
	if strings.Contains(uri, "git@") {
		re = regexp.MustCompile(".*git@(.*):([0-9]*\\/+)?(.*)\\.git")
		res := re.FindStringSubmatch(uri)
		pl.gitlabHost = res[1]
		pl.port = strings.Trim(res[2], "/")
		pl.projectPath = res[3]

	} else if strings.Index(uri, "http") == 0 {
		re = regexp.MustCompile("(https?):\\/\\/([^\\/]*)\\/(.*)\\.git")
		res := re.FindStringSubmatch(uri)
		pl.protocol = res[1]
		pl.gitlabHost = res[2]
		pl.projectPath = res[3]
	} else {
		err = fmt.Errorf("The url protocol is not supported: %s", uri)
	}

	return
}

func configureSslVerification() (err error) {
	if pl.Source.SkipSslVerification {
		err = os.Setenv("GIT_SSL_NO_VERIFY", "true")
		if err != nil {
			return
		}

		err = ioutil.WriteFile(os.ExpandEnv("HOME/.curlrc"), []byte("insecure"), 0644)
		if err != nil {
			return
		}
	}
	return
}

func exitIfErr(err error) {
	if err != nil {
		log.Fatalf("\nError at %s:\n", getCallerInfo())
	}
}

func exitIfErrMsg(err error, msg string) {
	if err != nil {
		log.Fatalf("\nError at %s:\n%s\n", getCallerInfo(), msg)
	}
}

func getCallerInfo() string {
	fpcs := make([]uintptr, 1)
	runtime.Callers(3, fpcs)
	caller := runtime.FuncForPC(fpcs[0] - 1)
	file, line := caller.FileLine(fpcs[0] - 1)
	return fmt.Sprintf("%s(%d)", file, line)
}
