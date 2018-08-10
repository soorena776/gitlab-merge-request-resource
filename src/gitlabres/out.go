package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
)

func out(sourceFolder string) *map[string]*Version {

	if len(pl.Params.Repository) == 0 {
		panic("please specify a repository")
	}
	if len(pl.Params.Status) == 0 {
		panic("please specify a status")
	}
	if len(pl.Source.ConcourseHost) == 0 {
		panic("please specify the concourse host address. (format url:port)")
	}
	if len(pl.Params.BuildLabel) == 0 {
		pl.Params.BuildLabel = defaultBuildLabel
	}

	targetVersionBytes, err := ioutil.ReadFile(fmt.Sprintf("%s/%s/%s", sourceFolder, pl.Params.Repository, versionFile))
	exitIfErr(err)
	targetVersion := Version{}
	exitIfErr(json.Unmarshal(targetVersionBytes, &targetVersion))

	targetURL := fmt.Sprintf("%s/teams/%s/pipelines/%s/jobs/%s/builds/%s",
		pl.Source.ConcourseHost,
		url.PathEscape(os.Getenv("BUILD_TEAM_NAME")),
		url.PathEscape(os.Getenv("BUILD_PIPELINE_NAME")),
		url.PathEscape(os.Getenv("BUILD_JOB_NAME")),
		url.PathEscape(os.Getenv("BUILD_NAME")))

	bodyJSON, err := json.Marshal(map[string]interface{}{
		"name":        pl.Params.BuildLabel,
		"state":       pl.Params.Status,
		"target_url":  targetURL,
		"description": targetVersion.BuildNum,
	})
	exitIfErr(err)

	header := map[string]string{
		"Content-Type": "application/json",
	}

	sendAPIRequestFunc("POST", "statuses/"+targetVersion.SHA, bodyJSON, header)

	return &map[string]*Version{"version": &targetVersion}
}
