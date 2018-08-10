package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	funk "github.com/thoas/go-funk"
)

var mr []*MergeRequest
var cs []*CommitStatus

func TestMain(m *testing.M) {
	Setup()
	retCode := m.Run()
	Teardown()
	os.Exit(retCode)
}

func Setup() {

}
func Teardown() {
}

func Test__CorrectParametersArePassedToGitlabApi(t *testing.T) {
	assert := assert.New(t)

	pl = &Payload{Source: Psource{URI: "uri"}}
	mr = []*MergeRequest{&MergeRequest{SHA: "somesha"}}
	cs = []*CommitStatus{&CommitStatus{Description: "Build 1", FinishedAt: "2018-08-09T14:46:33.940Z", Status: "success"}}
	sendAPIRequestFunc = func(method string, suburl string, body []byte, header map[string]string) (bytes []byte) {

		var expectedSuburl string
		if method != "GET" {
			panic("wrong method passed")
		}
		assert.Equal("GET", method)
		if strings.Contains(suburl, "statuses") {
			bytes, _ = json.Marshal(cs)
			expectedSuburl = fmt.Sprintf("repository/commits/%s/statuses", mr[0].SHA)
		} else if strings.Contains(suburl, "merge_request") {
			bytes, _ = json.Marshal(mr)
			expectedSuburl = "merge_requests?state=opened&order_by=updated_at"
		}

		if expectedSuburl != suburl {
			panic(fmt.Sprintf("wrong suburl passed: %s vs %s", expectedSuburl, suburl))
		}

		return bytes
	}

	resp := check()

	assert.Equal(0, len(resp))
}

func Test__NoMr_ReturnsEmptyVersionList(t *testing.T) {
	assert := assert.New(t)

	mockGitlabAPI()
	mr = nil
	cs = nil

	resp := check()

	assert.Equal(0, len(resp))
}

func Test__MrNeedsRebuildAndFreshMR_ReturnsExpected(t *testing.T) {
	assert := assert.New(t)

	mockGitlabAPI()

	pl = &Payload{Source: Psource{BuildExpiresAfter: "1h"}}
	mr = []*MergeRequest{
		&MergeRequest{SHA: "expired"},
		&MergeRequest{SHA: "notExpired"},
		&MergeRequest{SHA: "new1"},
		&MergeRequest{SHA: "new2"},
		&MergeRequest{SHA: "failed"},
	}
	cs = []*CommitStatus{
		&CommitStatus{
			Description: "Build 2", FinishedAt: time.Now().UTC().Add(-time.Hour).Format("2006-01-02T15:04:05.000Z"), Status: "success", SHA: "expired",
		},
		&CommitStatus{
			Description: "Build 4", FinishedAt: time.Now().UTC().Add(-time.Hour + time.Second).Format("2006-01-02T15:04:05.000Z"), Status: "success", SHA: "notExpired",
		},
		&CommitStatus{
			Description: "Build 1", FinishedAt: "2018-08-09T14:46:33.940Z", Status: "failed", SHA: "failed",
		},
	}

	resp := check()

	resMap := funk.Map(resp, func(v *Version) (string, string) {
		return v.SHA, v.BuildNum
	}).(map[string]string)

	//first build for the new merge request
	assert.Equal(resMap["new1"], "Build 1")

	//first build for the new merge request
	assert.Equal(resMap["new2"], "Build 1")

	//3rd build for the already successful merge request which is now expired
	assert.Equal(resMap["expired"], "Build 3")
}

func mockGitlabAPI() {
	sendAPIRequestFunc = func(method string, suburl string, body []byte, header map[string]string) []byte {

		var bytes []byte
		if strings.Contains(suburl, "statuses") {
			var re *regexp.Regexp
			re = regexp.MustCompile("repository\\/commits\\/(.*)\\/.*")
			res := re.FindStringSubmatch(suburl)
			cs := funk.Find(cs, func(c *CommitStatus) bool { return c.SHA == res[1] })
			if cs != nil {
				bytes, _ = json.Marshal([]*CommitStatus{cs.(*CommitStatus)})
			} else {
				bytes, _ = json.Marshal([]*CommitStatus{})
			}
			return bytes
		} else if strings.Contains(suburl, "merge_request") {
			bytes, _ := json.Marshal(mr)
			return bytes
		}

		return nil
	}
}
