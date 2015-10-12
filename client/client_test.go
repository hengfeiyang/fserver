package client

import (
	"cmstop-fserver/util"
	"fmt"
	"io/ioutil"
	"strconv"
	"testing"
	//"time"
)

var host = "127.0.0.1"
var port = "9468"
var password = "1234567890"

func TestClientWriteFile1(t *testing.T) {
	client, err := NewClient(host, port, password)
	if err != nil {
		t.Error(err)
	}
	defer client.Close()
	//body, err := ioutil.ReadFile("/Users/yanghengfei/Downloads/DEEPXTZJ_GHOST_XP_SP3_201311.iso")
	for i := 0; i < 1; i++ {
		err = client.WriteFile("/tmp/xxx/client"+strconv.Itoa(i), []byte("<-12345->"))
		if err != nil {
			t.Error(err)
		}
	}
}

func __TestClientWriteFile2(t *testing.T) {
	client, err := NewClient(host, port, password)
	if err != nil {
		t.Error(err)
	}
	defer client.Close()
	var baseDir = "/Users/yanghengfei/cmstop/SVN/cmstop"
	var newDir = "/tmp/x2"
	var list []string
	err = util.ReadDirRecursiveFiles(baseDir, &list)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(len(list))
	for i, f := range list {
		nf := newDir + f[len(baseDir):]
		fmt.Println(i, "\t", nf)
		body, err := ioutil.ReadFile(f)
		err = client.WriteFile(nf, body)
		if err != nil {
			t.Error(err)
		}
	}
}

func BenchmarkClient1(b *testing.B) {
	client, err := NewClient(host, port, password)
	if err != nil {
		b.Error(err)
	}
	defer client.Close()
	for i := 0; i < 1000; i++ {
		err = client.WriteFile("/tmp/xxx/client1", []byte("<-12345->"))
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkHello(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fmt.Sprintf("hello")
	}
}
