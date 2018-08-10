package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
)

func in(destFolder string) *map[string]*Version {

	setupGitCreds()
	cloneGitRepository(destFolder)

	//write version info the git folde
	targetVersion, err := json.Marshal(pl.Version)
	exitIfErr(err)
	exitIfErr(ioutil.WriteFile(fmt.Sprintf("%s/%s", destFolder, versionFile), targetVersion, 0644))

	mergeGitRepository(destFolder)

	return &map[string]*Version{"version": &pl.Version}
}

func setupGitCreds() {
	if len(pl.Source.PrivateKey) != 0 {
		rsaDir := os.ExpandEnv("$HOME/.ssh/")
		exitIfErr(os.MkdirAll(rsaDir, os.ModeDir|0744))
		exitIfErr(ioutil.WriteFile(rsaDir+"id_rsa", []byte(pl.Source.PrivateKey), 0500))

		pars := []string{"-t", "rsa", pl.gitlabHost}
		if len(pl.port) != 0 {
			pars = []string{"-t", "rsa", "-p", pl.port, pl.gitlabHost}
		}

		knownhost, err := exec.Command("ssh-keyscan", pars...).Output()
		exitIfErr(err)

		exitIfErr(ioutil.WriteFile(rsaDir+"/known_hosts", knownhost, 0500))
	} else {
		defLogin := fmt.Sprintf("default login %s password %s", pl.Source.Username, pl.Source.Password)
		exitIfErr(ioutil.WriteFile(os.ExpandEnv("$HOME/.netrc"), []byte(defLogin), 0644))
	}
}
