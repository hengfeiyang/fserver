package util

import (
	"testing"
)

func TestMail1(t *testing.T) {
	config := &MailT{
		"smtp.163.com:25",
		"zjwpub@163.com",
		"7EB74329@Zjw",
		"zjwpub@163.com",
		"safeie@163.com, yanghengfei@cmstop.com, zjwpub@163.com",
		"我是来测试的",
		"<h1>我是内容</h1>",
		"html",
	}
	err := SendMail(config)
	if err != nil {
		t.Error(err)
	}
}
