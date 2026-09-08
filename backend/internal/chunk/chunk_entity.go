package chunk

const ChunkSize = 5 << 20 // 5 MB

// ChunkUploadSession 分片上传会话结构体
// UploadedBits 布尔数组，下标代表分片序号，true代表该分片已经上传完成
// UploadID 上传会话唯一标识
// Filename 原始文件名
// FileSize 文件总大小(字节)
// ChunkSize 单个分片大小(字节)
// TotalChunks 分片总数量
// FileHash 整个文件的哈希值，用于秒传、完整性校验
type ChunkUploadSession struct {
	UploadID     string
	AccountID    uint
	Filename     string
	FileSize     int64
	ChunkSize    int64
	TotalChunks  int
	FileHash     string
	UploadedBits []bool // 分片上传标记位，数组下标 = 分片index
}

// UploadedChunks 获取已经上传成功的分片下标列表
// return: 已上传分片index数组，例如 [0,1,3] 代表0、1、3分片已上传，2分片缺失
func (s *ChunkUploadSession) UploadedChunks() []int {
	var indices []int
	for i, uploaded := range s.UploadedBits {
		if uploaded {
			indices = append(indices, i)
		}
	}
	return indices
}

// IsComplete 判断整个文件所有分片是否全部上传完毕
// return true:全部分片上传完成；false:存在未上传分片
func (s *ChunkUploadSession) IsComplete() bool {
	for _, b := range s.UploadedBits {
		if !b {
			return false
		}
	}
	return true
}

// InitChunkUploadRequest 初始化分片上传请求
// 客户端发起分片上传第一步：告知服务端文件基础信息，服务端生成upload_id会话
type InitChunkUploadRequest struct {
	Filename    string `json:"filename" binding:"required"`           // 原始文件名
	FileSize    int64  `json:"file_size" binding:"required,min=1"`    // 文件总大小，字节，必须大于0
	ChunkSize   int64  `json:"chunk_size" binding:"required,min=1"`   // 每个分片大小，字节
	TotalChunks int    `json:"total_chunks" binding:"required,min=1"` // 分片总个数
	FileHash    string `json:"file_hash" binding:"required"`          // 文件整体哈希，用于秒传、上传完成校验
}

// UploadChunkRequest 上传单个分片请求
// form表单参数，客户端上传分片二进制时携带，对应每一块分片上传接口
type UploadChunkRequest struct {
	UploadID   string `form:"upload_id" binding:"required"`  // 分片上传会话ID，初始化接口返回
	ChunkIndex int    `form:"chunk_index" binding:"min=0"`   // 当前分片下标，从0开始
	ChunkHash  string `form:"chunk_hash" binding:"required"` // 当前分片的哈希值，用于分片完整性校验
}

// ChunkStatusRequest 查询分片上传状态请求
// 客户端轮询查询哪些分片已经上传成功，用于断点续传，跳过已上传分片
type ChunkStatusRequest struct {
	UploadID string `json:"upload_id" binding:"required"` // 分片上传会话ID
}

// CompleteChunkUploadRequest 请求合并分片
// 客户端确认全部分片上传完成，调用该接口触发服务端分片合并、校验、生成完整文件
type CompleteChunkUploadRequest struct {
	UploadID string `json:"upload_id" binding:"required"` // 分片上传会话ID
}
