package client

import (
	"bytes"
	"cmstop-fserver/server"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/fatih/pool.v2"
	"net"
)

type FSClient struct {
	addr   *net.TCPAddr
	p      pool.Pool
	passwd string
}

var poolNum int

func NewClient(host, port, passwd string) (*FSClient, error) {
	addr, err := net.ResolveTCPAddr("tcp4", host+":"+port)
	if err != nil {
		return nil, err
	}
	client := new(FSClient)
	client.addr = addr
	p, err := pool.NewChannelPool(2, 100, func() (net.Conn, error) {
		poolNum = poolNum + 1
		fmt.Println("pool++ =", poolNum)
		return net.DialTCP("tcp4", nil, addr)
	})
	if err != nil {
		return nil, err
	}
	client.p = p
	client.passwd = passwd

	return client, nil
}

// 关闭连接
func (t *FSClient) Close() {
	t.p.Close()
}

// 向服务器写入数据
func sendData(conn net.Conn, method uint32, passwd string, path string, body []byte) error {
	msg := new(server.FileData)
	msg.Method = method
	msg.Password = passwd
	msg.Path = path
	msg.Body = body
	msg.PassLength = uint32(len(msg.Password))
	msg.PathLength = uint32(len(msg.Path))
	msg.BodySize = uint32(len(msg.Body))

	var err error
	var n int
	err = binary.Write(conn, binary.LittleEndian, msg.Method)
	if err != nil {
		return errors.New("Send Data Method Error: " + err.Error())
	}
	err = binary.Write(conn, binary.LittleEndian, msg.PassLength)
	if err != nil {
		return errors.New("Send Data PassLength Error: " + err.Error())
	}
	err = binary.Write(conn, binary.LittleEndian, msg.PathLength)
	if err != nil {
		return errors.New("Send Data PathLength Error: " + err.Error())
	}
	err = binary.Write(conn, binary.LittleEndian, msg.BodySize)
	if err != nil {
		return errors.New("Send Data BodySize Error: " + err.Error())
	}
	n, err = conn.Write([]byte(msg.Password))
	n, err = conn.Write([]byte(msg.Path))
	n, err = conn.Write(msg.Body)
	if err != nil || n == 0 {
		return errors.New("Send Data Error: " + err.Error())
	}
	return nil
}

// 从服务器端收字符串
func readData(conn net.Conn) error {
	buf := make([]byte, 4)
	num, err := conn.Read(buf[0:])
	result := bytes.NewBuffer(nil)
	result.Write(buf[0:num])
	var length uint32
	err = binary.Read(result, binary.LittleEndian, &length)
	if err != nil {
		return errors.New("Read Length Error: " + err.Error())
	}
	data := make([]byte, length)
	num, err = conn.Read(data)
	if err != nil || num != int(length) {
		return errors.New("Read Data Error: " + err.Error())
	}
	msg := new(server.ResponseData)
	err = json.Unmarshal(data, &msg)
	if msg.Code > 0 {
		return errors.New("Error: " + msg.Message)
	}
	return nil
}

// 写入文件
func (t *FSClient) WriteFile(path string, body []byte) error {
	conn, err := t.p.Get()
	//conn, err := net.DialTCP("tcp4", nil, t.addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = sendData(conn, server.METHOD_CREATE_FILE, t.passwd, path, body)
	if err != nil {
		return err
	}
	err = readData(conn)
	if err != nil {
		return err
	}
	return nil
}
