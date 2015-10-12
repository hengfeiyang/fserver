// Copyright (c) 2015 Beijing CmsTop Technology Co.,Ltd. (http://www.cmstop.com)

/*
Package server provider file server Daemon.
this file provider file operation.

header use LittleEndian binary data, body use byte.

receive protocol:
|-----------|-----------|-----------|-----------|-----------|-----------|-----------|
| 4b(uint)  | 4b(uint)  | 4b(uint)  | 4b(uint)  | N         | N         | N         |
|-----------|-----------|-----------|-----------|-----------|-----------|-----------|
| 操作类型  | 密钥长度  | 路径长度  | 内容长度  | 密钥      | 文件路径  | 文件内容  |
|-----------|-----------|-----------|-----------|-----------|-----------|-----------|
16byte header, 4uint. then password,path,body.
if method is copy or rename, body is json encode params.

response protocol:
|-----------|-----------------------------------------------------------------------|
| 4b(uint)  | N                                                                     |
|-----------|-----------------------------------------------------------------------|
| 消息长度  | 消息内容(JSON)                                                        |
|-----------|-----------------------------------------------------------------------|
4byte header, 1uint, then message.
*/
package server

import (
	"bytes"
	"cmstop-fserver/util"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/9466/goconfig"
	"io"
	"log"
	"net"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	MAX_PATH_LENGTH uint32 = 1 << 10   // 路径最大长度2014
	MAX_BODY_SIZE   uint32 = 1 << 30   // 硬编码限制大小 1024 * 1024 * 1024 1G
	DEF_IP                 = "0.0.0.0" // 默认监听IP
	DEF_PORT               = "9468"    // 默认监听端口
)

var (
	DENY_EXT = [4]string{"php", "cgi", "pl", "py"} // 禁止使用的扩展名
	DENY_DIR = [2]string{"/etc/", "/boot/"}        // 禁止写入的目录
)

// 服务器结构
type Server struct {
	Logger     *log.Logger      // 日志操作句柄
	Chs        chan uint32      // 和主进程通信的信号
	ConnNum    uint32           // 当前连接数
	Shutdown   bool             // 关闭信号
	listenIp   string           // 监听IP
	listenPort string           // 监听端口
	listener   *net.TCPListener // TCP处理句柄
	debug      bool             // 是否开启调试模式
	rootDir    []string         // 允许操作的目录
	allowExt   []string         // 允许运行使用的文件扩展名
	maxSize    uint32           // 允许操作的最大文件大小
	password   string           // 密钥
	mail       *mailConf        // 邮件配置信息
	ctFile     *CTFile          // 文件操作句柄
}

// 邮件发送信息结构
type mailConf struct {
	host       string
	user       string
	pass       string
	from       string
	to         string
	title      string
	serverInfo string
}

func NewServer() *Server {
	s := new(Server)
	s.Chs = make(chan uint32, 3)
	return s
}

func (s *Server) Init(conf *goconfig.ConfigFile) error {
	ip, err := conf.GetString("common", "listen")
	if err != nil || ip == "" {
		s.listenIp = DEF_IP
	} else {
		s.listenIp = ip
	}
	port, err := conf.GetString("common", "port")
	if err != nil || ip == "" {
		s.listenPort = DEF_PORT
	} else {
		s.listenPort = port
	}
	rootDir, err := conf.GetString("common", "rootDir")
	if err != nil || rootDir == "" {
		return errors.New("rootDir cannot empty")
	}
	s.rootDir = strings.Split(rootDir, ",")
	maxSize, err := conf.GetString("common", "maxSize")
	if maxSize == "" {
		s.maxSize = MAX_BODY_SIZE
	} else {
		s.maxSize = min(uint32(util.ReverseFormatSize(maxSize)), MAX_BODY_SIZE)
	}
	allowExt, err := conf.GetString("common", "allowExt")
	if err != nil || allowExt == "" {
		return errors.New("allowExt cannot empty")
	}
	s.allowExt = strings.Split(allowExt, ",")

	s.password, _ = conf.GetString("common", "password")
	s.debug, _ = conf.GetBool("log", "debug")

	s.mail = new(mailConf)
	s.mail.host, _ = conf.GetString("mail", "mailHost")
	s.mail.user, _ = conf.GetString("mail", "mailUser")
	s.mail.pass, _ = conf.GetString("mail", "mailPass")
	s.mail.from, _ = conf.GetString("mail", "mailFrom")
	s.mail.to, _ = conf.GetString("mail", "mailTo")
	s.mail.title, _ = conf.GetString("mail", "mailTitle")
	s.mail.serverInfo, _ = conf.GetString("mail", "serverInfo")

	// 初始化CTFile
	s.ctFile = NewCtFile(s)

	return nil
}

func (s *Server) Start() {
	addr, err := net.ResolveTCPAddr("tcp4", s.listenIp+":"+s.listenPort)
	if err != nil {
		s.Logger.Fatalln(err)
	}
	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		s.Logger.Fatalln(err)
	}
	s.listener = listener
	s.Logger.Println("CmsTop File Server begin serve.")
	for {
		if s.Shutdown == true {
			if s.ConnNum == 0 {
				s.Logger.Println("active connections serve done, now beginning shutdown...")
				break
			} else {
				s.Logger.Println("CmsTop File Server has", s.ConnNum, "actvie connections, serve continue...")
			}
			time.Sleep(100 * time.Millisecond) // 如果还在处理，等待1000毫秒继续
			continue
		}
		if s.debug {
			s.Logger.Println("CmsTop File Server new accept...")
		}
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			if oerr, ok := err.(*net.OpError); ok && oerr.Err.Error() == "use of closed network connection" {
				/* this hack happens because the error is returned when the
				 * network socket is closing and instead of returning a
				 * io.EOF it returns this error.New(...) struct.
				 */
				continue // 这是listener被关闭，不记录日志
			}
			s.Logger.Println(err.Error())
			continue // 输出异常，继续提供服务
		}
		s.ConnNum++ // 每启动一个处理，连接数+1
		go s.handle(conn)
	}
	s.Logger.Println("CmsTop File Server has been shutdown.")
	s.Chs <- 1 // 程序终止，写入Channel数据
}

func (s *Server) Stop() {
	s.listener.Close()
	s.Chs <- 1 // TODO 将写入和读取分离
	s.Chs <- 1 // TODO 过滤器，检测注入和木马
}

func (s *Server) handle(conn *net.TCPConn) {
	if s.debug {
		s.Logger.Println("Conn Acccpt", conn)
		s.Logger.Println("Client", conn.RemoteAddr().String())
	}
	defer func() {
		if s.debug {
			s.Logger.Println("Conn Closed", conn)
		}
		conn.Close()
		s.ConnNum-- // 每结束一个处理，连接数-1
	}()
	var err error
	var idleTime time.Time
	for {
		err = s.handleRequest(conn)
		if err != nil {
			if err == io.EOF { // io.EOF对方传输终止，关闭了连接，暂时没有数据
				if time.Now().After(idleTime.Add(time.Second * 10)) { // 空闲超时
					break
				}
				continue
			}
			s.Logger.Println(err)
			ClientWrite(conn, []byte(err.Error()), 1)
			break
		} else {
			ClientWrite(conn, []byte("success"), 0)
			// 每次处理成功后重置超时时间
			idleTime = time.Now()
			if s.debug {
				s.Logger.Println("Conn IdleTime Reset", conn)
			}
		}
		time.Sleep(5 * time.Millisecond) // 如果还在处理，等待5毫秒继续
	}
}

func (s *Server) handleRequest(conn *net.TCPConn) error {
	// 接收数据
	//conn.SetReadDeadline(time.Now().Add(time.Second * 30)) // 3秒超时
	rec, err := TCPConnRead(conn)
	if err != nil {
		if err == io.EOF {
			return err
		}
		return errors.New("TCPConnRead Data Error: " + err.Error())
	}

	// 判断密钥
	if len(s.password) > 0 && s.password != rec.Password {
		return errors.New("Password check failed")
	}

	// 判断路径
	err = s.CheckPath(rec.Path)
	if err != nil {
		return err
	}

	// DEBUG
	if s.debug {
		fmt.Println("conn: ", conn)
		fmt.Println("method: ", rec.Method)
		fmt.Println("path: ", rec.Path)
		fmt.Println("bodySize: ", rec.BodySize)
	}

	// 处理文件操作
	return s.ctFile.Handle(rec)
}

// 检测路径是否合法
func (s *Server) CheckPath(filepath string) error {
	// -- 判断是否是绝对路径
	if path.IsAbs(filepath) == false {
		return errors.New("Path must be absolute")
	}

	// -- 判断路径是否在黑名单中
	for _, d := range DENY_DIR {
		if strings.HasPrefix(filepath, d) {
			return errors.New("Path in DENY_DIR")
		}
	}

	// -- 判断文件扩展名是否在黑名单中
	ext := path.Ext(filepath)
	if ext != "" {
		for _, e := range DENY_EXT {
			if ext == "."+e {
				return errors.New("Extension in DENY_EXT")
			}
		}
	}

	// -- 判断路径是否在可允许的范围
	_dirAllow := false
	for _, d := range s.rootDir {
		if strings.HasPrefix(filepath, d) {
			_dirAllow = true
			break
		}
	}
	if _dirAllow == false {
		return errors.New("Path not in rootDir")
	}

	return nil
}

// 响应客户端信息
func ClientWrite(conn *net.TCPConn, msg []byte, code int) {
	send := new(ResponseData)
	send.Code = code
	send.Message = string(msg)
	json, _ := json.Marshal(send)
	length := uint32(len(json))
	binary.Write(conn, binary.LittleEndian, length)
	conn.Write(json)
}

// 协议封装读取
func TCPConnRead(conn *net.TCPConn) (*FileData, error) {
	rec := new(FileData)
	result := bytes.NewBuffer(nil)
	// 读取操作类型
	data := make([]byte, 16)
	num, err := conn.Read(data[0:4])
	if err != nil || num != 4 {
		if err == io.EOF {
			return nil, err
		}
		if err == nil {
			err = errors.New("method read error")
		}
		return nil, err
	}
	var method uint32
	result.Write(data[0:4])
	err = binary.Read(result, binary.LittleEndian, &method)
	if err != nil {
		return nil, err
	}
	rec.Method = method
	if int(rec.Method) <= METHOD_MIN || int(rec.Method) >= METHOD_MAX {
		return nil, errors.New("method not defined")
	}

	// 读取密钥长度
	num, err = conn.Read(data[4:8])
	if err != nil || num != 4 {
		if err == nil {
			err = errors.New("password length read error")
		}
		return nil, err
	}
	result.Reset()
	result.Write(data[4:8])
	err = binary.Read(result, binary.LittleEndian, &rec.PassLength)
	if err != nil {
		return nil, err
	}
	if rec.PassLength > 64 {
		return nil, errors.New("password legnth too large! password length should less than 64")
	}

	// 读取路径长度
	num, err = conn.Read(data[8:12])
	if err != nil || num != 4 {
		if err == nil {
			err = errors.New("path length read error")
		}
		return nil, err
	}
	result.Reset()
	result.Write(data[8:12])
	err = binary.Read(result, binary.LittleEndian, &rec.PathLength)
	if err != nil {
		return nil, err
	}
	if rec.PathLength > MAX_PATH_LENGTH {
		return nil, errors.New("path legnth too large! path length should less than " + strconv.FormatInt(int64(MAX_PATH_LENGTH), 10))
	}

	// 读取内容长度
	num, err = conn.Read(data[12:16])
	if err != nil || num != 4 {
		if err == nil {
			err = errors.New("body size read error")
		}
		return nil, err
	}
	result.Reset()
	result.Write(data[12:16])
	err = binary.Read(result, binary.LittleEndian, &rec.BodySize)
	if err != nil {
		return nil, err
	}
	if rec.BodySize > MAX_BODY_SIZE {
		return nil, errors.New("body too large! body should less than " + strconv.FormatInt(int64(MAX_BODY_SIZE), 10))
	}

	// 读取密钥
	_password := make([]byte, rec.PassLength)
	num, err = io.ReadFull(conn, _password)
	if err != nil {
		return nil, err
	}
	rec.Password = string(_password)

	// 读取操作路径
	_path := make([]byte, rec.PathLength)
	num, err = io.ReadFull(conn, _path)
	if err != nil {
		return nil, err
	}
	rec.Path = string(_path)

	// 读取内容
	if rec.BodySize == 0 {
		rec.Body = make([]byte, 0)
	} else {
		rec.Body = make([]byte, rec.BodySize)
		num, err = io.ReadFull(conn, rec.Body)
		if err != nil {
			return nil, err
		}
	}
	return rec, nil
}

func min(a, b uint32) uint32 {
	if a >= b {
		return b
	} else {
		return a
	}
}

func max(a, b uint32) uint32 {
	if a >= b {
		return a
	} else {
		return b
	}
}
