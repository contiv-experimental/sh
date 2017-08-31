// Copyright (c) 2016, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/contiv-experimental/sh/syntax"
)

type action uint

const (
	None action = iota
	Modify
	Error
)

var walkTests = []struct {
	want       action
	mode       os.FileMode
	path, body string
}{
	{Modify, 0666, "shebang-1", "#!/bin/sh\n foo"},
	{Modify, 0666, "shebang-2", "#!/bin/bash\n foo"},
	{Modify, 0666, "shebang-3", "#!/usr/bin/sh\n foo"},
	{Modify, 0666, "shebang-4", "#!/usr/bin/env bash\n foo"},
	{Modify, 0666, "shebang-5", "#!/bin/env sh\n foo"},
	{Modify, 0666, "shebang-space", "#! /bin/sh\n foo"},
	{Modify, 0666, "shebang-tabs", "#!\t/bin/env\tsh\n foo"},
	{Modify, 0666, "shebang-args", "#!/bin/bash -e -x\nfoo"},
	{Modify, 0666, "ext.sh", " foo"},
	{Modify, 0666, "ext.bash", " foo"},
	{Modify, 0666, "ext-shebang.sh", "#!/bin/sh\n foo"},
	{Modify, 0666, filepath.Join("dir", "ext.sh"), " foo"},
	{None, 0666, ".hidden", " foo long enough"},
	{None, 0666, ".hidden-shebang", "#!/bin/sh\n foo"},
	{None, 0666, "..hidden-shebang", "#!/bin/sh\n foo"},
	{None, 0666, "noext-empty", " foo"},
	{None, 0666, "noext-noshebang", " foo long enough"},
	{None, 0666, "shebang-nonewline", "#!/bin/shfoo"},
	{None, 0666, "ext.other", " foo"},
	{None, 0666, "ext-shebang.other", "#!/bin/sh\n foo"},
	{None, 0666, "shebang-nospace", "#!/bin/envsh\n foo"},
	{None, 0666, filepath.Join(".git", "ext.sh"), " foo"},
	{None, 0666, filepath.Join(".svn", "ext.sh"), " foo"},
	{None, 0666, filepath.Join(".hg", "ext.sh"), " foo"},
	{Error, 0666, "parse-error.sh", " foo("},
	{Error, 0111, "open-error.sh", " foo"},
}

var errPathMentioned = regexp.MustCompile(`([^ :]+):`)

func TestWalk(t *testing.T) {
	parser = syntax.NewParser(syntax.KeepComments)
	printer = syntax.NewPrinter()
	dir, err := ioutil.TempDir("", "shfmt-walk")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	for _, wt := range walkTests {
		if dir, _ := filepath.Split(wt.path); dir != "" {
			dir = dir[:len(dir)-1]
			os.Mkdir(dir, 0777)
		}
		err := ioutil.WriteFile(wt.path, []byte(wt.body), wt.mode)
		if err != nil {
			t.Fatal(err)
		}
	}
	var outBuf bytes.Buffer
	out = &outBuf
	*list, *write = true, true
	*simple = true
	gotError := false
	errored := map[string]bool{}
	onError := func(err error) {
		gotError = true
		line := err.Error()
		if sub := errPathMentioned.FindStringSubmatch(line); sub != nil {
			errored[sub[1]] = true
		}
	}
	doWalk := func(path string) {
		gotError = false
		outBuf.Reset()
		walk(path, onError)
	}
	doWalk(".")
	modified := map[string]bool{}
	outScan := bufio.NewScanner(&outBuf)
	for outScan.Scan() {
		path := outScan.Text()
		modified[path] = true
	}
	for _, wt := range walkTests {
		t.Run(wt.path, func(t *testing.T) {
			mod := modified[wt.path]
			if mod && wt.want != Modify {
				t.Fatalf("walk had to not run on %s but did", wt.path)
			} else if !mod && wt.want == Modify {
				t.Fatalf("walk had to run on %s but didn't", wt.path)
			}
			err := errored[wt.path]
			if err && wt.want != Error {
				t.Fatalf("walk had to not err on %s but did", wt.path)
			} else if !err && wt.want == Error {
				t.Fatalf("walk had to err on %s but didn't", wt.path)
			}
		})
	}
	if doWalk("."); outBuf.Len() > 0 {
		t.Fatal("shfmt -l -w printed paths on a duplicate run")
	}
	*list, *write = false, false
	if doWalk("."); outBuf.Len() == 0 {
		t.Fatal("shfmt without -l nor -w did not print anything")
	}
	if doWalk(".hidden"); outBuf.Len() == 0 {
		t.Fatal("`shfmt .hidden` did not print anything")
	}
	if doWalk("nonexistent"); !gotError {
		t.Fatal("`shfmt nonexistent` did not error")
	}
	if err := ioutil.WriteFile("nowrite", []byte(" foo"), 0444); err != nil {
		t.Fatal(err)
	}
	*write = true
	if doWalk("nowrite"); !gotError {
		t.Fatal("`shfmt nowrite` did not error")
	}
}
