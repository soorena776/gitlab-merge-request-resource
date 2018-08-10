package main

type Payload struct {
	Source  Psource `json:"source"`
	Version Version `json:"version"`
	Params  Params  `json:"params"`

	//runtime variables
	gitlabAPIbase string `json:"-"`
	gitlabHost    string `json:"-"`
	port          string `json:"-"`
	projectPath   string `json:"-"`
	protocol      string `json:"-"`
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
	BuildExpiresAfter   string `json:"build_expires_after"`
}

type CommitStatus struct {
	Status      string `json:"status"`
	FinishedAt  string `json:"finished_at"`
	Description string `json:"description"`
	SHA         string `json:"sha"`
}

type MergeRequest struct {
	SHA string `json:"sha"`
}

type Version struct {
	SHA      string `json:"sha"`
	BuildNum string `json:"build_num"`
}

type Params struct {
	Repository string `json:"repository"`
	Status     string `json:"status"`
	BuildLabel string `json:"build_label"`
}

const defaultBuildLabel = "Concourse"
const minimumBuildExpiration = 1
const versionFile = "/target_version.txt"
