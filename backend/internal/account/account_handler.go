package account

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"feed/backend/internal/apierror"
	"github.com/gin-gonic/gin"
)

// 头像上传相关配置
const (
	avatarUploadDir = "uploads/avatars" // 头像本地保存目录
	maxAvatarBytes  = 5 << 20           // 单张头像大小上限 5MB
)

// allowedAvatarExts 允许的头像图片扩展名白名单
var allowedAvatarExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
}

type AccountHandler struct {
	accountService *AccountService
}

func NewAccountHandler(accountService *AccountService) *AccountHandler {
	return &AccountHandler{accountService: accountService}
}

func getAccountID(c *gin.Context) (uint, error) {
	value, exists := c.Get("accountID")
	if !exists {
		return 0, errors.New("accountID not found")
	}
	id, ok := value.(uint)
	if !ok {
		return 0, errors.New("accountID has invalid type")
	}
	return id, nil
}

func (h *AccountHandler) CreateAccount(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if err := h.accountService.CreateAccount(c.Request.Context(), &Account{
		Username: req.Username,
		Password: req.Password,
	}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "account created"})
}

func (h *AccountHandler) Rename(c *gin.Context) {
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	token, err := h.accountService.Rename(c.Request.Context(), accountID, req.NewUsername)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"token": token})
}

func (h *AccountHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if err := h.accountService.ChangePassword(c.Request.Context(), req.Username, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(400, gin.H{"error": "unsuccessfully password changed"})
		return
	}
	c.JSON(200, gin.H{"message": "successfully password changed"})
}

func (h *AccountHandler) FindByID(c *gin.Context) {
	var req FindByIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if account, err := h.accountService.FindByID(c.Request.Context(), req.ID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	} else {
		c.JSON(200, account)
	}
}

func (h *AccountHandler) FindByUsername(c *gin.Context) {
	var req FindByUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if account, err := h.accountService.FindByUsername(c.Request.Context(), req.Username); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	} else {
		c.JSON(200, account)
	}
}

func (h *AccountHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	account, err := h.accountService.FindByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	accessToken, refreshToken, err := h.accountService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, LoginResponse{Token: accessToken, RefreshToken: refreshToken, AccountID: account.ID, Username: account.Username})
}

func (h *AccountHandler) Logout(c *gin.Context) {
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if err := h.accountService.Logout(c.Request.Context(), accountID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "account logged out"})
}

// UploadAvatar 上传头像：接收 multipart 文件，校验并落盘后持久化可访问 URL
func (h *AccountHandler) UploadAvatar(c *gin.Context) {
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	// 限制请求体大小，防止超大请求占用磁盘
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarBytes+1<<20)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的头像文件（file 字段，不超过 5MB）"})
		return
	}
	if fileHeader.Size > maxAvatarBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像文件大小不能超过 5MB"})
		return
	}

	// 仅允许常见图片扩展名，避免任意类型文件落盘
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedAvatarExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 jpg/jpeg/png/webp/gif 格式的头像"})
		return
	}

	if err := os.MkdirAll(avatarUploadDir, 0o755); err != nil {
		slog.Error("创建头像目录失败", "dir", avatarUploadDir, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务内部错误"})
		return
	}
	// 使用账号 ID 与时间戳生成唯一文件名，避免路径穿越与相互覆盖
	fileName := fmt.Sprintf("avatar_%d_%d%s", accountID, time.Now().UnixNano(), ext)
	dst := filepath.Join(avatarUploadDir, fileName)
	if err := c.SaveUploadedFile(fileHeader, dst); err != nil {
		slog.Error("保存头像文件失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务内部错误"})
		return
	}

	// 构造静态访问 URL 并落库；生产环境建议替换为对象存储返回的公网地址
	avatarURL := "/static/avatars/" + fileName
	if err := h.accountService.UpdateAvatar(c.Request.Context(), accountID, avatarURL); err != nil {
		slog.Error("更新头像地址失败", "account_id", accountID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务内部错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": avatarURL, "message": "头像上传成功"})
}

// UpdateProfile 更新个人资料（头像/简介非必填，只更新传入的字段）
func (h *AccountHandler) UpdateProfile(c *gin.Context) {
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if err := h.accountService.UpdateProfile(c.Request.Context(), accountID, &req); err != nil {
		// 没有任何字段可更新视为参数错误
		if errors.Is(err, ErrInvalidParam) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		slog.Error("更新个人资料失败", "account_id", accountID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "个人资料更新成功"})
}

func (h *AccountHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	newToken, accountID, username, err := h.accountService.RefreshAccessToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	c.JSON(http.StatusOK, LoginResponse{Token: newToken, AccountID: accountID, Username: username})
}
