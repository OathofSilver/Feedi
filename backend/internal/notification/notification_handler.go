package notification

import (
	"feed/backend/internal/apierror"
	"feed/backend/internal/utils/jwt"
	"github.com/gin-gonic/gin"
	"net/http"
)

// Handler 通知 HTTP 接口
type Handler struct {
	service *Service
}

// NewHandler 创建通知处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List 查询当前登录用户的通知列表（JWT 中间件已注入 accountID）。
// 请求体可选 {before_time, limit}；before_time=0 表示首页。
func (h *Handler) List(c *gin.Context) {
	var req ListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	recipientID, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	resp, err := h.service.ListByRecipient(c.Request.Context(), recipientID, req.BeforeTime, limit)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if resp.Notifications == nil {
		resp.Notifications = []ListItem{}
	}
	c.JSON(http.StatusOK, resp)
}
