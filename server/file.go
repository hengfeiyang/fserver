package server

import (
	"cmstop-fserver/util"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path"
)

type copyParams struct {
	NewPath string `json:newpath`
}

type renameParams struct {
	NewPath string `json:newpath`
}

type CTFile struct {
	Server *Server     // server实例指针
	Logger *log.Logger // 日志实例指针
}

func NewCtFile(server *Server) *CTFile {
	f := new(CTFile)
	f.Server = server
	f.Logger = server.Logger
	return f
}

// 处理请求
func (t *CTFile) Handle(rec *FileData) error {
	switch int(rec.Method) {
	case METHOD_CREATE_FILE:
		return t.WriteFile(rec.Path, rec.Body, false)
	case METHOD_MODIFY_FILE:
		return t.WriteFile(rec.Path, rec.Body, false)
	case METHOD_APPEND_FILE:
		return t.WriteFile(rec.Path, rec.Body, true)
	case METHOD_REMOVE_FILE:
		return t.RemoveFile(rec.Path)
	case METHOD_CREATE_DIR:
		return t.CreateDir(rec.Path)
	case METHOD_REMOVE_DIR:
		return t.RemoveDir(rec.Path)
	case METHOD_CLEAR_DIR:
		return t.ClearDir(rec.Path)
	case METHOD_COPY:
		return t.Copy(rec.Path, rec.Body)
	case METHOD_RENAME:
		return t.Rename(rec.Path, rec.Body)
	}
	return errors.New("Method not defined")
}

// 创建文件，修改文件，追加写入
func (t *CTFile) WriteFile(path string, body []byte, isAppend bool) error {
	err := checkPath(path)
	if err != nil {
		return err
	}
	var f *os.File
	if isAppend { // 追加模式
		f, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0664)
	} else { // 覆写模式
		f, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0664)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(body)
	if err != nil {
		return err
	}
	return nil
}

// 删除文件
func (t *CTFile) RemoveFile(path string) error {
	if ok, _ := util.IsExist(path); !ok {
		return nil // 文件不存在，直接返回
	}
	return os.Remove(path)
}

// 创建目录
func (t *CTFile) CreateDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// 删除目录，含子目录中的内容和目录本身
func (t *CTFile) RemoveDir(path string) error {
	if ok, _ := util.IsExist(path); !ok {
		return nil // 文件不存在，直接返回
	}
	return os.RemoveAll(path)
}

// 清空目录，只清空目录中的内容含子目录，但目录本身不删除
func (t *CTFile) ClearDir(path string) error {
	if ok, _ := util.IsExist(path); !ok {
		return nil // 文件不存在，直接返回
	}
	err := os.RemoveAll(path)
	if err != nil {
		return err
	}
	return os.Mkdir(path, 0755)
}

// 复制一个路径，文件或文件夹，如果是文件夹递归复制所有子目录中的内容
// body是params的json编码后的数据
func (t *CTFile) Copy(path string, body []byte) error {
	params := new(copyParams)
	err := json.Unmarshal(body, &params)
	if params.NewPath == "" {
		return errors.New("destination path error: cannot empty")
	}
	err = t.Server.CheckPath(params.NewPath)
	if err != nil {
		return errors.New("destination path error: " + err.Error())
	}
	err = checkPath(params.NewPath)
	if err != nil {
		return err
	}

	f, err := os.Stat(path)
	if err != nil {
		return err
	}
	if f.IsDir() {
		err = util.CopyDir(path, params.NewPath)
	} else {
		err = util.CopyFile(path, params.NewPath)
	}
	return err
}

// 更名，重命名一个路径，文件或文件夹，如果不存在返回错误
// body是params的json编码后的数据
func (t *CTFile) Rename(path string, body []byte) error {
	params := new(renameParams)
	err := json.Unmarshal(body, &params)
	if params.NewPath == "" {
		return errors.New("destination path error: cannot empty")
	}
	err = t.Server.CheckPath(params.NewPath)
	if err != nil {
		return errors.New("destination path error: " + err.Error())
	}
	err = checkPath(params.NewPath)
	if err != nil {
		return err
	}
	return os.Rename(path, params.NewPath)
}

// 检测路径是否存在并可写
func checkPath(p string) error {
	dir := path.Dir(p)
	if ok, err := util.IsExist(dir); ok == false {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return errors.New("permission deny, cannot mkdir: " + dir)
		}
	} else {
		if ok, _ := util.IsWritable(dir); ok == false {
			return errors.New("permission deny, dir not allow write: " + dir)
		}
	}
	return nil
}
