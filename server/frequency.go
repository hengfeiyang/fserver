// Copyright (c) 2015 Beijing CmsTop Technology Co.,Ltd. (http://www.cmstop.com)

/*
file write frequency monitor and warning.
*/
package server

import (
	"cmstop-fserver/util"
	"fmt"
	"github.com/9466/daemon"
	"github.com/9466/goconfig"
	"github.com/go-fsnotify/fsnotify"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DEF_WPS   = 100 // 默认阀值，每秒写入文件大于这个值，就报警
	DEF_DELAY = 3   // 延迟发送，等待延时，如果在n秒内没有新的变化，认为该次处理完成，发送报告
)

type MonitorItem struct {
	event string
	name  string
	time  time.Time
}

type monitor struct {
	mail     *mailConf     // 邮件配置信息
	dirs     []string      // 监控的目录
	wps      int           // 预警阀值
	wpsWarn  bool          // 阀值预警报告，是否发送，超过阀值即触发
	debug    bool          // 是否调试模式
	items    []MonitorItem // 被修车或创建的文件列表
	logger   *log.Logger   // 日志处理句柄
	markTime time.Time     // 标记开始时间
	lastTime time.Time     // 最后写入时间
}

// 全局实例化
var mon *monitor

func newFrequency() {
	// Config
	config := parseConfig()
	isDaemon, err := config.GetBool("log", "daemon")
	logfile, err := config.GetString("log", "logfile")

	// Daemon
	if isDaemon {
		_, err := daemon.Daemon(1, 0)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// Log
	var logFileHandle *os.File
	if isDaemon {
		if logfile != "" {
			logFileHandle, err = os.OpenFile(logfile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
		} else {
			logFileHandle, err = os.OpenFile("/dev/null", 0, 0)
		}
	} else {
		logFileHandle = os.Stderr
	}
	defer logFileHandle.Close()
	if err != nil {
		log.Fatalln(err.Error())
	}
	mon.logger = log.New(logFileHandle, "", log.Ldate|log.Ltime)

	// Watch
	initWatcher()
}

func parseConfig() *goconfig.ConfigFile {
	baseDir, err := util.GetDir()
	if err != nil {
		log.Fatalln("basedir get error: " + err.Error())
	}
	configFile := baseDir + "/config/cmstop.conf"
	conf, err := goconfig.ReadConfigFile(configFile)
	if err != nil {
		log.Fatalln("ReadConfigFile Err: ", err.Error(), "\nConfigFile:", configFile)
	}

	mon = new(monitor)
	mon.items = make([]MonitorItem, 0)
	mondir, err := conf.GetString("common", "monDir")
	mon.dirs = strings.Split(mondir, ",")
	mon.wps, _ = conf.GetInt("common", "wps")
	if mon.wps == 0 {
		mon.wps = DEF_WPS
	}
	mon.debug, _ = conf.GetBool("log", "debug")

	mon.mail = &mailConf{}
	mon.mail.host, _ = conf.GetString("mail", "mailHost")
	mon.mail.user, _ = conf.GetString("mail", "mailUser")
	mon.mail.pass, _ = conf.GetString("mail", "mailPass")
	mon.mail.from, _ = conf.GetString("mail", "mailFrom")
	mon.mail.to, _ = conf.GetString("mail", "mailTo")
	mon.mail.title, _ = conf.GetString("mail", "mailTitle")
	mon.mail.serverInfo, _ = conf.GetString("mail", "serverInfo")

	return conf
}

func initWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		mon.logger.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					mon.logger.Println("modified file:", event.Name)
					handleWatcher("MODIFY", event.Name)
				} else if event.Op&fsnotify.Create == fsnotify.Create {
					mon.logger.Println("created file:", event.Name)
					handleWatcher("CREATE", event.Name)
				}
			case err := <-watcher.Errors:
				mon.logger.Println("error:", err)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-time.After(DEF_DELAY * time.Second):
				resetWatcher()
			}
		}
	}()

	for _, dir := range mon.dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		l := make([]string, 0)
		l = append(l, dir)
		l, err = util.RecursiveDir(dir, l)
		if err != nil {
			mon.logger.Fatal(err)
		}
		for _, d := range l {
			mon.logger.Println("watcher dir: ", d)
			err = watcher.Add(d)
			if err != nil {
				mon.logger.Fatal(err)
			}
		}
	}
	<-done
}

func handleWatcher(event, name string) {
	item := MonitorItem{
		event: event,
		name:  name,
		time:  time.Now(),
	}
	mon.items = append(mon.items, item)
	mon.lastTime = item.time
	if mon.markTime.IsZero() {
		mon.markTime = item.time
		if mon.debug {
			mon.logger.Println("set markTime")
		}
	} else {
		if mon.wpsWarn {
			return
		}
		//mon.logger.Println("check wpsWarn")
		if mon.markTime.Unix()-mon.lastTime.Unix() == 0 && len(mon.items) >= mon.wps {
			body := "<p>发生时间：" + mon.markTime.Format("2006-01-02 15:04:05") + "<br/>" +
				"异常信息：在1秒内超过" + strconv.Itoa(mon.wps) + "次文件创建或修改<br/>" +
				"服务信息：" + mon.mail.serverInfo + "</p>"
			err := mail(body)
			if err != nil {
				mon.logger.Println(err)
			} else {
				mon.logger.Println("mail notifaction send.")
				mon.wpsWarn = true
			}
		}
	}
}

func resetWatcher() {
	if mon.debug {
		mon.logger.Println("reset called.")
		mon.logger.Println("items", len(mon.items), "wpsWarn", mon.wpsWarn,
			"markTime", mon.markTime.IsZero(), "lastTime", mon.lastTime.IsZero())
	}
	if len(mon.items) == 0 {
		return
	}
	if mon.lastTime.Unix()+DEF_DELAY > time.Now().Unix() {
		return
	}
	if mon.wpsWarn == false {
		if len(mon.items) > 0 {
			mon.items = make([]MonitorItem, 0)
			mon.markTime = time.Time{}
			mon.lastTime = time.Time{}
		}
		return
	}

	body := "<p>发生时间：" + mon.markTime.Format("2006-01-02 15:04:05") + "<br/>" +
		"结束时间：" + mon.lastTime.Format("2006-01-02 15:04:05") + "<br/>" +
		"异常信息：在过去" + strconv.Itoa(int(mon.lastTime.Unix()-mon.markTime.Unix())) + "秒内共产生" + strconv.Itoa(len(mon.items)) + "次文件创建或修改<br/>" +
		"服务信息：" + mon.mail.serverInfo + "</p>"
	lis := ""
	for i, item := range mon.items {
		lis += "<li style=\"list-style:none\">" + strconv.Itoa(i+1) + ". " + item.time.Format("2006-01-02 15:04:05") + " " + item.event + " " + item.name + "</li>"
	}
	body += "<ul><h3>文件列表</h3>" + lis + "</ul>"
	err := mail(body)
	if err != nil {
		mon.logger.Println(err)
	} else {
		mon.logger.Println("mail full report send.")
	}

	mon.wpsWarn = false
	mon.items = make([]MonitorItem, 0)
	mon.markTime = time.Time{}
	mon.lastTime = time.Time{}
}

func mail(body string) error {
	mon.logger.Println("send mail begin.")
	conf := &util.MailT{
		mon.mail.host,
		mon.mail.user,
		mon.mail.pass,
		mon.mail.from,
		mon.mail.to,
		mon.mail.title,
		body,
		"html",
	}
	err := util.SendMail(conf)
	mon.logger.Println("send mail end.")
	return err
}
