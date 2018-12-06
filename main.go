package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func main() {
	defer glog.Flush()
	glog.CopyStandardLogTo("info")
	runtime.GOMAXPROCS(runtime.NumCPU())

	var message string
	var versionString string
	flag.StringVar(&message, "message", "", "Message used for commit and changelog")
	flag.StringVar(&versionString, "version", "", "Version used for commit and changelog")

	_ = flag.Set("logtostderr", "true")
	flag.Parse()

	version, err := ParseVersion(versionString)
	if err != nil {
		if glog.V(2) {
			glog.Fatalf("%+v", err)
		} else {
			glog.Fatal(err)
		}
	}

	if err := bumpVersion(message, *version); err != nil {
		if glog.V(2) {
			glog.Fatalf("%+v", err)
		} else {
			glog.Fatal(err)
		}
	}

	glog.V(0).Infof("done")
}

func ParseVersion(versionNumber string) (*Version, error) {
	var err error
	parts := strings.Split(versionNumber, ".")
	if len(parts) != 3 {
		return nil, errors.New("expect version in format 1.2.3")
	}
	version := &Version{}
	version.Major, err = strconv.Atoi(parts[0])
	if err != nil {
		return nil, err
	}
	version.Minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}
	version.Patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return nil, err
	}
	return version, nil
}

type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

var re = regexp.MustCompile(`(?s)^(.*?)(\n##\s.*)$`)

func bumpVersion(message string, version Version) error {
	pwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "get working directory failed")
	}
	r, err := git.PlainOpen(pwd)
	if err != nil {
		return errors.Wrap(err, "open git directory failed")
	}

	w, err := r.Worktree()
	if err != nil {
		return errors.Wrap(err, "get worktree failed")
	}

	status, err := w.Status()
	if err != nil {
		return errors.Wrap(err, "get status failed")
	}

	clean := status.IsClean()
	if !clean {
		return errors.New("git repo is not clean")
	}

	_, err = r.Tag(version.String())
	if err == nil {
		return errors.New("tag already exists")
	}
	glog.V(2).Infof("tag not found")

	changelogFilename := "CHANGELOG.md"
	changelog, err := ioutil.ReadFile(changelogFilename)
	if err != nil {
		changelog = []byte(defaultChangelog)
	}

	changelog = re.ReplaceAll(changelog, []byte(fmt.Sprintf("$1\n## %s\n\n- %s\n$2", version.String(), message)))
	err = ioutil.WriteFile(changelogFilename, changelog, 0600)
	if err != nil {
		return errors.Wrap(err, "write CHANGELOG.md failed")
	}
	glog.V(2).Infof("changelog updated")

	_, err = w.Add(changelogFilename)
	if err != nil {
		return errors.Wrap(err, "add file failed")
	}

	signature := &object.Signature{
		Name:  "Benjamin Borbe",
		Email: "bborbe@rocketnews.de",
		When:  time.Now(),
	}
	commit, err := w.Commit(message, &git.CommitOptions{
		Author: signature,
	})

	obj, err := r.CommitObject(commit)
	if err != nil {
		return errors.Wrap(err, "commit failed")
	}

	_, err = r.CreateTag(version.String(), obj.Hash, &git.CreateTagOptions{
		Tagger:  signature,
		Message: version.String(),
	})
	if err != nil {
		return errors.Wrap(err, "create tag failed")
	}

	return nil
}

const defaultChangelog = `# Changelog

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## 1.0.0

- Initial Version
`
