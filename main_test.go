package main

import (
	"path"
	"testing"
)

func TestName(t *testing.T) {
	var target = "./target/"
	var file = "./target/1.txt"
	// 将file 去除target后的路径
	file = file[len(target):]
	match, err := path.Match(target, file)
	if err != nil {
		return
	}
	if match {
		t.Log("match")
	} else {
		t.Log("not match")
	}
}

func TestUtils(t *testing.T) {
	t.Log(path.Base("1.txt"))
}
