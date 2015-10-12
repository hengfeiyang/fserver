package util

import (
	"testing"
)

func TestCopyDir(t *testing.T) {
	err := CopyDir("/Users/yanghengfei/cmstop/SVN/cmstop/framework", "/tmp/x3")
	if err != nil {
		t.Error(err)
	}
}
