// Copyright (c) 2018 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	flag "github.com/bborbe/flagenv"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func main() {
	defer glog.Flush()
	glog.CopyStandardLogTo("info")
	runtime.GOMAXPROCS(runtime.NumCPU())

	app := &App{}
	flag.StringVar(&app.AuthorName, "git-author-name", "", "Author Name")
	flag.StringVar(&app.AuthorEmail, "git-author-email", "", "Author Email")
	flag.StringVar(&app.Message, "message", "", "Message used for commit and changelog")
	flag.StringVar(&app.Version, "version", "", "Version used for commit and changelog")
	flag.StringVar(&app.Repo, "repo", "", "Git Repo")

	_ = flag.Set("logtostderr", "true")
	flag.Parse()

	if app.Repo == "" {
		var err error
		app.Repo, err = os.Getwd()
		if err != nil {
			glog.Fatal(err)
		}
	}

	if err := app.Validate(); err != nil {
		glog.Fatal(err)
	}

	if err := app.Run(context.Background()); err != nil {
		if glog.V(2) {
			glog.Fatalf("%+v", err)
		} else {
			glog.Fatal(err)
		}
	}

	glog.V(0).Infof("done")
}

type App struct {
	Message     string
	Version     string
	AuthorName  string
	AuthorEmail string
	Repo        string
}

func (a *App) Validate() error {
	if a.Message == "" {
		return errors.New("Message missing")
	}
	if a.Version == "" {
		return errors.New("Version missing")
	}
	if a.AuthorName == "" {
		return errors.New("AuthorName missing")
	}
	if a.AuthorEmail == "" {
		return errors.New("AuthorEmail missing")
	}
	if a.Repo == "" {
		return errors.New("Repo missing")
	}
	return nil
}

func (a *App) Run(ctx context.Context) error {
	version, err := ParseVersion(a.Version)
	if err != nil {
		return err
	}

	r, err := git.PlainOpen(a.Repo)
	if err != nil {
		return errors.Wrap(err, "open git directory failed")
	}

	w, err := r.Worktree()
	if err != nil {
		return errors.Wrap(err, "get worktree failed")
	}

	_, err = r.Tag(version.String())
	if err == nil {
		return errors.New("tag already exists")
	}
	glog.V(2).Infof("tag not found")

	changelog, err := ioutil.ReadFile(changelogFilename)
	if err != nil {
		changelog = []byte(defaultChangelog)
	}

	changelog = re.ReplaceAll(changelog, []byte(fmt.Sprintf("$1\n## %s\n\n- %s\n$2", version.String(), a.Message)))
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
		Name:  a.AuthorName,
		Email: a.AuthorEmail,
		When:  time.Now(),
	}
	commit, err := w.Commit(a.Message, &git.CommitOptions{
		All:    true,
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

const changelogFilename = "CHANGELOG.md"

const defaultChangelog = `# Changelog

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## 1.0.0

- Initial Version
`
