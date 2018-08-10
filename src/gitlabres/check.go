package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func check() []*Version {

	var openMRsSHA []*MergeRequest
	resp := sendAPIRequestFunc("GET", "merge_requests?state=opened&order_by=updated_at", nil, nil)
	exitIfErr(json.Unmarshal(resp, &openMRsSHA))

	needABuild := []*Version{}
	// find out the first merge request that needs a build
	for _, mr := range openMRsSHA {

		resp := sendAPIRequestFunc("GET", fmt.Sprintf("repository/commits/%s/statuses", mr.SHA), nil, nil)
		var commitStatuses []*CommitStatus
		exitIfErrMsg(json.Unmarshal(resp, &commitStatuses), "Unable to unmarshal merge request the response")
		if len(commitStatuses) == 0 {
			// no builds before for this commit. needs a build
			needABuild = append(needABuild, &Version{SHA: mr.SHA, BuildNum: "Build 1"})
		} else if num := nextBuildIfExpired(commitStatuses[0]); num != "" {
			needABuild = append(needABuild, &Version{SHA: mr.SHA, BuildNum: num})
		}
	}

	return needABuild
}

//nextBuildIfExpired returns an integer for the next build number given a commit build status. -1 means no new build is needed
func nextBuildIfExpired(commitStatus *CommitStatus) string {

	// first see if it has previously succeeded. No need to rebuild an already failing commit
	if commitStatus.Status != "success" || pl.Source.BuildExpiresAfter == "" {
		return ""
	}

	finishedAt := parseTime(commitStatus.FinishedAt)

	// then see if the given build expiration period is valid
	expDuration, err := time.ParseDuration(pl.Source.BuildExpiresAfter)
	exitIfErrMsg(err, "Not a valid duration string. refer to https://golang.org/pkg/time/#ParseDuration")
	if (minimumBuildExpiration * time.Minute) > expDuration {
		exitIfErrMsg(fmt.Errorf(""), fmt.Sprintf("the build expiration cannot be less than 5 minutes. Its currently set at %s", pl.Source.BuildExpiresAfter))
	}

	// then see if the build is expired
	if finishedAt.Add(expDuration).Before(time.Now().UTC()) {
		num := "Build 2"
		if strings.Contains(commitStatus.Description, "Build ") {
			lastBuildNum, err := strconv.Atoi(commitStatus.Description[len("Build "):])
			if err == nil {
				num = fmt.Sprintf("Build %d", lastBuildNum+1)
			}
		}

		return num
	}

	return ""
}
