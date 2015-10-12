package server

// 定义文件操作代码
const (
	METHOD_MIN         = iota // 标识，用来判断method的范围
	METHOD_CREATE_FILE = iota // 创建文件，当文件不存在时，尝试创建
	METHOD_MODIFY_FILE        // 修改文件，当文件不存在时，返回错误，当文件存在时从头部开始写入
	METHOD_APPEND_FILE        // 增量写入文件，当文件不存在时，尝试创建，当文件存在时从尾部追加写入
	METHOD_REMOVE_FILE        // 删除文件，当文件不存在时，返回错误
	METHOD_CREATE_DIR         // 创建目录
	METHOD_REMOVE_DIR         // 删除目录，含子目录中的内容和目录本身
	METHOD_CLEAR_DIR          // 清空目录，只清空目录中的内容含子目录，但目录本身不删除
	METHOD_COPY               // 复制一个路径，文件或文件夹，如果是文件夹递归复制所有子目录中的内容
	METHOD_RENAME             // 更名，重命名一个路径，文件或文件夹，如果不存在返回错误
	METHOD_MAX                // 标识，用来判断method的范围
)

// 交互数据结构
type FileData struct {
	Method     uint32 // 操作方法，是一组定义的枚举常量
	PassLength uint32 // 密钥长度
	PathLength uint32 // 路径长度
	BodySize   uint32 // 内容长度
	Password   string // 密钥
	Path       string // 操作路径
	Body       []byte // 文件内容，允许为空
}

// 响应数据结构
type ResponseData struct {
	Code    int    `json:"code"`    // 状态码，0 表示成功，非0表示失败
	Message string `json:"message"` // 消息字符串
}
