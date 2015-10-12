// Copyright (c) 2015 Beijing CmsTop Technology Co.,Ltd. (http://www.cmstop.com)

/*
Package cmstop-fserver provides file server Daemon.
used for file create, modify, delete, clear operations,
it's contain injection filter and extension filter.
it's can warning adm with email or other method.
protocol details in wiki http://wiki.cmstop.dev/cmstop-fserver
*/
package main

import (
	"cmstop-fserver/server"
	"cmstop-fserver/util"
	"fmt"
	"github.com/9466/daemon"
	"github.com/9466/goconfig"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	NAME           = "CmsTop File Server"
	VERSION        = "1.0 beta"
	CONFIG_DIR     = "conf/"          // 配置文件目录，因为有多个配置文件，所以指定目录，不指定文件
	CONFIG_FILE    = "cmstop.conf"    // 主配置文件
	INJECTION_FILE = "injection.conf" // 木马检测配置文件
	PROCESS_NUM    = 3                // 进程数量，1 文件写入，2 文件读取， 3，文件监测
)

func main() {
	// parse args

	var err error
	var confdir, pidfile string
	argc := len(os.Args)
	for key, val := range os.Args {
		switch val {
		case "-C":
			if argc > key+1 {
				confdir = os.Args[key+1]
			}
		case "--confdir":
			if argc > key+1 {
				confdir = os.Args[key+1]
			}
		case "-P":
			if argc > key+1 {
				pidfile = os.Args[key+1]
			}
		case "--pid":
			if argc > key+1 {
				pidfile = os.Args[key+1]
			}
		case "-V":
			version()
			os.Exit(0)
		case "--version":
			version()
			os.Exit(0)
		case "-h":
			help()
			os.Exit(0)
		case "--help":
			help()
			os.Exit(0)
		}
	}

	// Config
	if confdir == "" {
		confdir, err = util.GetDir()
		if err != nil {
			log.Fatalln("basedir get error: " + err.Error())
		}
		confdir += "/" + CONFIG_DIR
	} else {
		confdir = fixPath(confdir) + "/"
	}
	configFile := confdir + CONFIG_FILE
	config, err := goconfig.ReadConfigFile(configFile)
	if err != nil {
		log.Fatalln("ReadConfigFile Err: ", err.Error(), "\nConfigFile:", configFile)
	}

	isDaemon, err := config.GetBool("log", "daemon")
	logfile, err := config.GetString("log", "logFile")

	// pidfile
	pidfile = fixPath(pidfile)
	if pidfile != "" {
		if ok, _ := util.IsExist(pidfile); !ok {
			f, err := os.OpenFile(pidfile, os.O_CREATE, 0666)
			if err != nil {
				log.Fatalln("pid file cannot create: " + pidfile)
			}
			f.Close()
		} else {
			if ok, _ := util.IsWritable(pidfile); !ok {
				log.Fatalln("pid file cannot write: " + pidfile)
			}
		}
	}

	// Daemon
	if isDaemon {
		_, err := daemon.Daemon(1, 0)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// pid
	if pidfile != "" {
		pid := os.Getpid()
		err := ioutil.WriteFile(pidfile, []byte(strconv.Itoa(pid)), 0666)
		if err != nil {
			log.Fatalln("pid " + pidfile + err.Error())
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

	s := server.NewServer()
	s.Logger = log.New(logFileHandle, "", log.Ldate|log.Ltime)
	// init之前必须先设置logger
	err = s.Init(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 开始服务
	s.Logger.Println("CmsTop File Server starting...")
	go s.Start()

	// 监听系统信号，重启或停止服务
	// trap signal
	sch := make(chan os.Signal, 10)
	signal.Notify(sch, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGINT,
		syscall.SIGHUP, syscall.SIGSTOP, syscall.SIGQUIT)
	go func(ch <-chan os.Signal) {
		sig := <-ch
		s.Logger.Println("signal recieved " + sig.String() + ", at: " + time.Now().String())
		s.Shutdown = true
		s.Stop()
		if sig == syscall.SIGHUP {
			s.Logger.Println("CmsTop File Server restart now...")
			procAttr := new(os.ProcAttr)
			procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
			procAttr.Dir = os.Getenv("PWD")
			procAttr.Env = os.Environ()
			process, err := os.StartProcess(os.Args[0], os.Args, procAttr)
			if err != nil {
				s.Logger.Println("CmsTop File Server restart process failed:" + err.Error())
				return
			}
			waitMsg, err := process.Wait()
			if err != nil {
				s.Logger.Println("CmsTop File Server restart wait error:" + err.Error())
			}
			s.Logger.Println(waitMsg)
		} else {
			s.Logger.Println("CmsTop File Server shutdown now...")
		}
	}(sch)

	// 启动chs监听，等待所有Process都运行结束
	for i := 0; i < PROCESS_NUM; i++ {
		<-s.Chs
	}
	s.Logger.Println("CmsTop File Server stopped.")
}

func fixPath(path string) string {
	var dir string
	var err error
	dir, err = util.GetDir()
	if err != nil {
		return ""
	}

	if path == "" {
		return path
	}

	if path[0] == '.' && path[1] == '/' {
		path = dir + "/" + path[2:]
	}
	if path[0] != '/' {
		path = dir + "/" + path
	}
	return strings.TrimRight(path, "/")
}

func help() {
	prog := path.Base(os.Args[0])
	fmt.Println(NAME, VERSION)
	fmt.Println("")
	fmt.Println("this is cmstop file server, provider file operations.")
	fmt.Println("")
	fmt.Println("Usage: " + prog + " [OPTIONS]")
	fmt.Println("  -C|--confdir <dir> \t config dir, default " + CONFIG_DIR)
	fmt.Println("  -P|--pid <file> \t pid file, default none.")
	fmt.Println("  -h|--help \t\t Output this help and exit. ")
	fmt.Println("  -V|--version \t\t Output version and and exit. ")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  " + prog + " -P /var/run/" + prog + ".pid -L /var/log/" + prog + ".log")
	fmt.Println("")
}

func version() {
	fmt.Println(NAME, VERSION)
	fmt.Println("")
}
