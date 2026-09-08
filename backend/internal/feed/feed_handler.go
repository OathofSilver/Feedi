package feed

import (
	"feed/backend/internal/apierror"
	"feed/backend/internal/utils/jwt"
	"github.com/gin-gonic/gin"
	"time"
)

type FeedHandler struct {
	service *FeedService
}

func NewFeedHandler(service *FeedService) *FeedHandler {
	return &FeedHandler{service: service}
}

func (f *FeedHandler) ListLatest(c *gin.Context) {
	var req ListLatestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}
	//数据库中存储时间的类型 都是time.Time
	var latestTime time.Time
	if req.LatestTime > 0 {
		latestTime = time.UnixMilli(req.LatestTime) // 若前端传秒，需改为 Unix，否则会错误
	}
	// 4. 从 JWT 中获取当前登录用户 ID，若未登录或获取失败则视为游客（ID=0）
	viewerAccountID, err := jwt.GetAccountID(c)
	if err != nil {
		viewerAccountID = 0
	}
	// 5. 调用业务服务层获取 Feed 流数据（包含是否已点赞等用户状态）
	feedItems, err := f.service.ListLatest(c.Request.Context(), req.Limit, latestTime, viewerAccountID)
	if err != nil {
		// 服务层错误统一返回 500，实际生产应细化错误类型
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// 6. 防御性处理：过滤掉 VideoList 中的 nil 元素，防止 JSON 序列化异常或前端空指针
	feedItems.VideoList = nonNilFeedVideoItems(feedItems.VideoList)
	c.JSON(200, feedItems)
}

func (f *FeedHandler) ListLikesCount(c *gin.Context) {
	var req ListLikesCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}
	var cursor *LikesCountCursor
	if req.LikesCountBefore != nil || req.IDBefore != nil {
		if req.LikesCountBefore == nil || req.IDBefore == nil {
			c.JSON(400, gin.H{"error": "likes_count_before and id_before must be provided together"})
			return
		}

		likesCountBefore := *req.LikesCountBefore
		idBefore := *req.IDBefore

		if likesCountBefore < 0 {
			c.JSON(400, gin.H{"error": "invalid cursor: likes_count_before must be >= 0"})
			return
		}
		if idBefore == 0 {
			if likesCountBefore != 0 {
				c.JSON(400, gin.H{"error": "invalid cursor: id_before must be > 0"})
				return
			}
		} else {
			cursor = &LikesCountCursor{
				LikesCount: likesCountBefore,
				ID:         idBefore,
			}
		}
	}
	viewerAccountID, err := jwt.GetAccountID(c)
	if err != nil {
		viewerAccountID = 0
	}
	feedItems, err := f.service.ListLikesCount(c.Request.Context(), req.Limit, cursor, viewerAccountID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	feedItems.VideoList = nonNilFeedVideoItems(feedItems.VideoList)
	c.JSON(200, feedItems)
}

func (f *FeedHandler) ListByFollowing(c *gin.Context) {
	var req ListByFollowingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}
	viewerAccountID, err := jwt.GetAccountID(c)
	if err != nil {
		viewerAccountID = 0
	}
	var latestTime time.Time
	if req.LatestTime > 0 {
		latestTime = time.Unix(req.LatestTime, 0)
	}
	feedItems, err := f.service.ListByFollowing(c.Request.Context(), req.Limit, latestTime, viewerAccountID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	feedItems.VideoList = nonNilFeedVideoItems(feedItems.VideoList)
	c.JSON(200, feedItems)
}

func (f *FeedHandler) ListByPopularity(c *gin.Context) {
	var req ListByPopularityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}
	viewerAccountID, err := jwt.GetAccountID(c)
	if err != nil {
		viewerAccountID = 0
	}

	var latestPopularity int64
	var latestBefore time.Time
	var latestIDBefore uint

	if req.LatestPopularity < 0 {
		c.JSON(400, gin.H{"error": "latest_popularity must be >= 0"})
		return
	}

	anyCursor := !req.LatestBefore.IsZero() || req.LatestIDBefore != nil
	if anyCursor {
		if req.LatestBefore.IsZero() || req.LatestIDBefore == nil || *req.LatestIDBefore == 0 {
			c.JSON(400, gin.H{"error": "latest_before and latest_id_before must be provided together"})
			return
		}
		latestPopularity = req.LatestPopularity
		latestBefore = req.LatestBefore
		latestIDBefore = *req.LatestIDBefore
	}
	resp, err := f.service.ListByPopularity(
		c.Request.Context(),
		req.Limit,
		req.AsOf,
		req.Offset,
		viewerAccountID,
		latestPopularity,
		latestBefore,
		latestIDBefore,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	resp.VideoList = nonNilFeedVideoItems(resp.VideoList)
	c.JSON(200, resp)
}

// nonNilFeedVideoItems 处理Feed视频切片，保证返回值永远不为nil
// 如果传入的items为nil，则返回空切片；非nil直接原样返回
// 目的：避免上层遍历切片时出现nil切片，防止range nil（虽然range nil安全，但统一返回空切片减少判nil逻辑）
func nonNilFeedVideoItems(items []FeedVideoItem) []FeedVideoItem {
	if items == nil {
		return []FeedVideoItem{}
	}
	return items
}

// ListByTag 根据标签获取视频Feed列表
func (h *FeedHandler) ListByTag(c *gin.Context) {
	var req struct {
		TagName string `json:"tag_name"` // 需要查询的标签名称
		Limit   int    `json:"limit"`    // 请求返回视频条数
	}
	// JSON参数绑定，校验请求体格式合法性
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// 校验标签名不能为空
	if req.TagName == "" {
		c.JSON(400, gin.H{"error": "tag_name is required"})
		return
	}

	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}

	viewerAccountID, _ := jwt.GetAccountID(c)

	items, err := h.service.ListByTag(c.Request.Context(), req.TagName, req.Limit, viewerAccountID)
	if err != nil {

		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	// nonNilFeedVideoItems：处理切片nil问题，保证前端始终收到数组[]而非null
	c.JSON(200, gin.H{"video_list": nonNilFeedVideoItems(items)})
}
