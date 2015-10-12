// Copyright (c) 2015 Beijing CmsTop Technology Co.,Ltd. (http://www.cmstop.com)

/*
Package server provider file server Daemon.
this file provider injection filter and warning.
*/
package server

import (
	"log"
)

type CTFilter struct {
	Logger *log.Logger // 日志操作句柄
}

func NewCtFilter(logger *log.Logger) *CTFilter {
	f := new(CTFilter)
	f.Logger = logger
	return f
}

func (t *CTFilter) Frequency(path string) {

}

func (t *CTFilter) Injection(path string) {

}
