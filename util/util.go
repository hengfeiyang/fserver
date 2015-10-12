// Copyright (c) 2013 Beijing CmsTop Technology Co.,Ltd. (http://www.cmstop.com)

/*
Package util provides support for format and read SFSS protocol.
*/
package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	RAND_CHARS      = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ" // 随机字符串的母本
	RAND_MAX_LENGTH = 64                                                               // 随机字符串最大长度
)

// aes加密
func AesEncrypt(data, iv, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	data = PKCS5Padding(data, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv)
	cryptData := make([]byte, len(data))
	blockMode.CryptBlocks(cryptData, data)
	baseData := make([]byte, base64.StdEncoding.EncodedLen(len(cryptData)))
	base64.StdEncoding.Encode(baseData, cryptData)
	return baseData, nil
}

// aes解密
func AesDecrypt(data, iv, key []byte) ([]byte, error) {
	baseData := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	length, err := base64.StdEncoding.Decode(baseData, data)
	if err != nil {
		return nil, err
	}
	data = baseData[:length]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode := cipher.NewCBCDecrypter(block, iv)
	blockSize := block.BlockSize()
	origData := make([]byte, len(data))
	blockMode.CryptBlocks(origData, data)
	origData, err = PKCS5UnPadding(origData, blockSize)
	if err != nil {
		return nil, err
	}
	return origData, nil
}

// aes加密补码
func PKCS5Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

// aes解密去码
func PKCS5UnPadding(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	unpadding := int(data[length-1])
	if unpadding >= blockSize {
		return nil, errors.New("AES PCKS5UnPadding penic, unpadding Illegal")
	}
	return data[:(length - unpadding)], nil
}

// 获取程序运行的目录
func GetDir() (string, error) {
	path, err := filepath.Abs(os.Args[0])
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

// 判断一个文件或目录是否存在
func IsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	// Check if error is "no such file or directory"
	if _, ok := err.(*os.PathError); ok {
		return false, nil
	}
	return false, err
}

// 判断一个文件或目录是否有写入权限
func IsWritable(path string) (bool, error) {
	err := syscall.Access(path, syscall.O_RDWR)
	if err == nil {
		return true, nil
	}
	// Check if error is "no such file or directory"
	if _, ok := err.(*os.PathError); ok {
		return false, nil
	}
	return false, err
}

// 生成一个随机字符串
func RandString(n int) string {
	if n > RAND_MAX_LENGTH {
		n = RAND_MAX_LENGTH
	}
	s := make([]byte, 0, n+1)
	sn := len(RAND_CHARS)
	rand.Seed(int64(time.Now().Nanosecond()))
	for i := 0; i < n; i++ {
		s = append(s, RAND_CHARS[rand.Intn(sn)])
	}
	return string(s)
}

// 格式化size单位 输出友好格式(B,KB,MB,GB,TB,PB)
func FormatSize(s int64) string {
	if s >= 1<<50 {
		f := float64(s) / (1 << 50)
		return fmt.Sprintf("%.2f PB", f)
	}
	if s >= 1<<40 {
		f := float64(s) / (1 << 40)
		return fmt.Sprintf("%.2f TB", f)
	}
	if s >= 1<<30 {
		f := float64(s) / (1 << 30)
		return fmt.Sprintf("%.2f GB", f)
	}
	if s >= 1<<20 {
		f := float64(s) / (1 << 20)
		return fmt.Sprintf("%.2f MB", f)
	}
	if s >= 1<<10 {
		f := float64(s) / (1 << 10)
		return fmt.Sprintf("%.2f KB", f)
	}
	return fmt.Sprintf("%d byte", s)
}

// 反向格式化size 将KB,MB,M,GB,G,TB,T 转化为byte
func ReverseFormatSize(s string) int64 {
	if s == "" {
		return 0
	}
	s = strings.ToUpper(s)
	var c byte
	var d int64
	var l int = len(s)
	if s[l-1:] == "B" { // 如果最后一位是B，去除
		s = s[0 : l-1]
	}
	l = len(s)
	if l == 1 { // 只有1位，直接转成数字返回
		d, _ = strconv.ParseInt(s, 10, 64)
		return d
	}
	// 判断后缀单位
	c = s[l-1:][0]
	d, _ = strconv.ParseInt(s[0:l-1], 10, 64)
	switch c {
	case 'P':
		return d * 1 << 50
	case 'T':
		return d * 1 << 40
	case 'G':
		return d * 1 << 30
	case 'M':
		return d * 1 << 20
	case 'K':
		return d * 1 << 10
	}
	// 没有单位，或单位不可识别，直接转成数字返回
	d, _ = strconv.ParseInt(s, 10, 64)
	return d
}

// 递归获取一个目录的子目录
func RecursiveDir(dir string, l []string) ([]string, error) {
	dl, err := ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, d := range dl {
		if d.IsDir() {
			_dir := dir + "/" + d.Name()
			l = append(l, _dir)
			l, err = RecursiveDir(_dir, l)
			if err != nil {
				return l, err
			}
		}
	}
	return l, err
}

// 读取一个文件夹返回文件列表
func ReadDir(dirname string) ([]os.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	return list, nil
}

// 读取一个文件夹返回文件列表(含子文件夹)
func ReadDirRecursiveFiles(dirname string, flist *[]string) error {
	f, err := os.Open(dirname)
	if err != nil {
		return err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return err
	}
	for _, f := range list {
		if f.IsDir() {
			err = ReadDirRecursiveFiles(dirname+"/"+f.Name(), flist)
			if err != nil {
				return err
			}
		} else {
			*flist = append(*flist, dirname+"/"+f.Name())
		}
	}
	return nil
}

// 复制一个文件，如果同名文件已存在，直接覆盖
func CopyFile(src, dest string) error {
	fs, err := os.OpenFile(src, os.O_RDONLY, 0664)
	if err != nil {
		return err
	}
	defer fs.Close()
	fd, err := os.OpenFile(dest, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = io.Copy(fd, fs)
	return err
}

// 复制一个文件夹，如果同名文件夹已存在，返回错误
func CopyDir(src, dest string) error {
	if ok, _ := IsExist(dest); ok {
		return errors.New(dest + " is exists")
	}
	srcinfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dest, srcinfo.Mode())
	if err != nil {
		return err
	}

	dir, err := os.Open(src)
	defer dir.Close()

	items, err := dir.Readdir(-1)
	for _, item := range items {
		srcNew := src + "/" + item.Name()
		destNew := dest + "/" + item.Name()
		if item.IsDir() {
			err = CopyDir(srcNew, destNew)
		} else {
			err = CopyFile(srcNew, destNew)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// 获取一个文件或目录的大小
func GetPathSize(path string) (int64, error) {
	var size int64 = 0
	s, err := os.Stat(path)
	if err != nil {
		return size, err
	}
	if s.IsDir() == true {
		fl, err := ReadDir(path)
		if err != nil {
			return size, err
		}
		for _, v := range fl {
			vs, err := GetPathSize(path + "/" + v.Name())
			if err != nil {
				return size, err
			}
			size += vs
		}
	} else {
		size += s.Size()
	}
	return size, nil
}
