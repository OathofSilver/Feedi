// API 进程入口：负责对外提供 HTTP 服务。
// 启动流程：加载配置 -> 连接 MySQL/Redis/RabbitMQ -> 组装生产者/仓库/服务/Handler
// -> 注册路由 -> 启动 Outbox 轮询器与 pprof -> 监听端口并优雅退出。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // 注册 pprof handler 到 DefaultServeMux
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"feed/backend/internal/account"
	"feed/backend/internal/comment"
	"feed/backend/internal/config"
	"feed/backend/internal/feed"
	"feed/backend/internal/like"
	"feed/backend/internal/middleware"
	"feed/backend/internal/notification"
	"feed/backend/internal/social"
	"feed/backend/internal/ssehub"
	"feed/backend/internal/utils/jwt"
	"feed/backend/internal/utils/mysql"
	"feed/backend/internal/utils/outboxpoller"
	"feed/backend/internal/utils/rabbitmq"
	rediscache "feed/backend/internal/utils/redis"
	"feed/backend/internal/video"
	"feed/backend/internal/worker/producer"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

// 雪花算法节点 ID：同一进程内各生产者节点必须唯一，跨进程也需与 worker 进程错开
const (
	nodeLikeMQ         = int64(1)
	nodePopularityMQ   = int64(2)
	nodeCommentMQ      = int64(3)
	nodeTimelineMQ     = int64(5)
	nodeNotificationMQ = int64(6)
)

// 视频详情缓存 TTL，与热度消费者清理缓存机制配合
const videoCacheTTL = 10 * time.Minute

func main() {
	if err := run(); err != nil {
		slog.Error("api 进程异常退出", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// ---------- 配置 ----------
	cfg, err := config.LoadDefault()
	if err != nil {
		return err
	}

	// ---------- 鉴权初始化：签发与验签共用配置密钥，必须在任何业务处理前完成 ----------
	middleware.InitAuth(cfg.JWT.Secret, cfg.JWT.ExpireHours, cfg.JWT.RefreshExpireHours)

	// ---------- 优雅退出：信号 -> ctx 取消 ----------
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	g, ctx := errgroup.WithContext(ctx)

	// ---------- 基础设施：MySQL / Redis / RabbitMQ ----------
	db, err := mysql.NewDB(cfg.Database)
	if err != nil {
		return err
	}
	// 建/补全全部数据表（EnsureDatabase 已收敛进 NewDB，库不存在会自动创建）。
	// 迁移失败视为致命：避免带不完整 schema 继续运行导致运行期 table doesn't exist。
	if err := mysql.AutoMigrate(db); err != nil {
		return err
	}
	defer func() {
		if err := mysql.CloseDB(db); err != nil {
			slog.Warn("关闭 MySQL 失败", "err", err)
		}
	}()

	cache, err := rediscache.NewFromEnv(&cfg.Redis)
	if err != nil {
		return err
	}
	defer func() {
		if err := cache.Close(); err != nil {
			slog.Warn("关闭 Redis 失败", "err", err)
		}
	}()

	baseMQ, err := rabbitmq.NewRabbitMQ(&cfg.RabbitMQ)
	if err != nil {
		return err
	}
	defer func() {
		if err := baseMQ.Close(); err != nil {
			slog.Warn("关闭 RabbitMQ 失败", "err", err)
		}
	}()

	// ---------- MQ 生产者（API 侧负责投递事件） ----------
	likeMQ, err := producer.NewLikeMQ(baseMQ, nodeLikeMQ)
	if err != nil {
		return err
	}
	popularityMQ, err := producer.NewPopularityMQ(baseMQ, nodePopularityMQ)
	if err != nil {
		return err
	}
	commentMQ, err := producer.NewCommentMQ(baseMQ, nodeCommentMQ)
	if err != nil {
		return err
	}
	// 社交关注不接入 MQ：social 服务同步落库 + 失效关注流缓存即可，
	// 关注流读 DB 实时可见，无需时间线写扩散
	timelineMQ, err := producer.NewTimelineMQ(baseMQ, nodeTimelineMQ)
	if err != nil {
		return err
	}

	// ---------- 仓库层 ----------
	accountRepo := account.NewAccountRepository(db)
	videoRepo := video.NewVideoRepository(db)
	likeRepo := like.NewLikeRepository(db)
	socialRepo := social.NewSocialRepository(db)
	feedRepo := feed.NewFeedRepository(db)
	commentRepo := comment.NewCommentRepository(db)

	// ---------- 通知模块 ----------
	// 通知数据可靠落库（与评论/点赞/关注同步写库一致），MQ 仅驱动实时 SSE 推送。
	notificationRepo := notification.NewRepository(db)
	notificationMQ, err := producer.NewNotificationMQ(baseMQ, nodeNotificationMQ)
	if err != nil {
		return err
	}
	defer func() {
		if err := notificationMQ.Close(); err != nil {
			slog.Warn("关闭 notification MQ 失败", "err", err)
		}
	}()

	// actor 用户名解析适配器：写入通知时用账号ID冗余用户名，避免读取时 join
	notifService := notification.NewService(notificationRepo, notificationMQ, acctNameResolver{repo: accountRepo})

	// SSE Hub：维护"账号->在线连接"，Broadcaster 消费通知队列后定向推送
	hub := ssehub.NewHub()
	notifyCh, err := baseMQ.NewChannel()
	if err != nil {
		return err
	}
	defer func() {
		if err := notifyCh.Close(); err != nil {
			slog.Warn("关闭 notification channel 失败", "err", err)
		}
	}()
	broadcaster, err := ssehub.NewBroadcaster(hub, notifyCh, cache)
	if err != nil {
		return err
	}
	defer func() {
		if err := broadcaster.Close(); err != nil {
			slog.Warn("关闭 notification broadcaster 失败", "err", err)
		}
	}()
	g.Go(func() error { return broadcaster.Run(ctx) })

	// ---------- 服务层 ----------
	accountService := account.NewAccountService(accountRepo, cache)
	videoService := video.NewVideoService(db, videoRepo, cache, videoCacheTTL, popularityMQ)
	likeService := like.NewLikeService(likeRepo, videoRepo, likeMQ, popularityMQ, cache, notifService)
	commentService := comment.NewCommentService(commentRepo, videoRepo, cache, commentMQ, popularityMQ, notifService)
	socialService := social.NewSocialService(socialRepo, accountRepo, cache, notifService)
	feedService := feed.NewFeedService(feedRepo, likeRepo, cache)

	// ---------- Handler 层 ----------
	accountHandler := account.NewAccountHandler(accountService)
	videoHandler := video.NewVideoHandler(videoService, accountService)
	likeHandler := like.NewLikeHandler(likeService)
	commentHandler := comment.NewCommentHandler(commentService, accountService)
	socialHandler := social.NewSocialHandler(socialService)
	feedHandler := feed.NewFeedHandler(feedService)
	notificationHandler := notification.NewHandler(notifService)

	// ---------- Outbox 轮询器：本地事务消息最终投递到 RabbitMQ ----------
	outboxpoller.StartOutboxPoller(db, timelineMQ)

	// ---------- HTTP 服务 ----------
	router := newRouter(cfg, cache, hub, accountHandler, videoHandler, likeHandler, commentHandler, socialHandler, feedHandler, notificationHandler)

	srv := &http.Server{
		Addr:         ":" + itoa(cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// pprof 性能分析端口，生产环境建议关闭
	if cfg.Observability.Pprof.Enabled && cfg.Observability.Pprof.APIAddr != "" {
		g.Go(func() error {
			slog.Info("pprof 已启动", "addr", cfg.Observability.Pprof.APIAddr)
			if err := http.ListenAndServe(cfg.Observability.Pprof.APIAddr, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		})
	}

	// 监听 HTTP 端口
	g.Go(func() error {
		slog.Info("api 进程启动", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	// 等待退出信号或任一组件出错
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	// 优雅关闭 HTTP 服务：给在途请求最多 10 秒处理时间
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("HTTP 服务关闭异常", "err", err)
	}
	slog.Info("api 进程已退出")
	return nil
}

// newRouter 组装 gin 路由与中间件、注册全部业务路由
func newRouter(
	cfg *config.Config,
	cache *rediscache.Client,
	hub *ssehub.Hub,
	accountHandler *account.AccountHandler,
	videoHandler *video.VideoHandler,
	likeHandler *like.LikeHandler,
	commentHandler *comment.CommentHandler,
	socialHandler *social.SocialHandler,
	feedHandler *feed.FeedHandler,
	notificationHandler *notification.Handler,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 头像等静态资源
	r.Static("/static/avatars", "uploads/avatars")

	// 健康检查
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")

	// ---------- 账号 ----------
	acc := api.Group("/account")
	{
		// 公开接口
		acc.POST("/register", accountHandler.CreateAccount)
		acc.POST("/login", accountHandler.Login)
		acc.POST("/refresh", accountHandler.Refresh)
		acc.POST("/info", accountHandler.FindByID)
		acc.POST("/username", accountHandler.FindByUsername)

		// 需登录
		authed := acc.Group("", jwt.JWTAuth(cache))
		{
			authed.POST("/logout", accountHandler.Logout)
			authed.POST("/rename", accountHandler.Rename)
			authed.POST("/password", accountHandler.ChangePassword)
			authed.POST("/profile", accountHandler.UpdateProfile)
			authed.POST("/avatar", accountHandler.UploadAvatar)
		}
	}

	// ---------- Feed 流：游客可浏览，登录后附带点赞等个人状态 ----------
	feedGroup := api.Group("/feed", jwt.SoftJWTAuth(cache))
	{
		feedGroup.POST("/latest", feedHandler.ListLatest)
		feedGroup.POST("/likes_count", feedHandler.ListLikesCount)
		feedGroup.POST("/following", feedHandler.ListByFollowing)
		feedGroup.POST("/popularity", feedHandler.ListByPopularity)
		feedGroup.POST("/tag", feedHandler.ListByTag)
	}

	// ---------- 视频 ----------
	videoGroup := api.Group("/video")
	{
		// 游客可看详情
		detail := videoGroup.Group("", jwt.SoftJWTAuth(cache))
		detail.POST("/detail", videoHandler.GetDetail)

		// 需登录
		authed := videoGroup.Group("", jwt.JWTAuth(cache))
		{
			authed.POST("/publish", videoHandler.PublishVideo)
			authed.POST("/upload", videoHandler.UploadVideo)
			authed.POST("/cover", videoHandler.UploadCover)
			authed.POST("/delete", videoHandler.DeleteVideo)
			authed.POST("/likes_count/update", videoHandler.UpdateLikesCount)
		}
	}

	// ---------- 点赞（需登录） ----------
	likeGroup := api.Group("/like", jwt.JWTAuth(cache))
	{
		likeGroup.POST("/action", likeHandler.Like)
		likeGroup.POST("/unlike", likeHandler.Unlike)
		likeGroup.POST("/is_liked", likeHandler.IsLiked)
		likeGroup.POST("/my_videos", likeHandler.ListMyLikedVideos)
	}

	// ---------- 评论 ----------
	commentGroup := api.Group("/comment")
	{
		// 游客可看评论列表
		commentGroup.POST("/list", commentHandler.GetAllComments)

		authed := commentGroup.Group("", jwt.JWTAuth(cache))
		{
			authed.POST("/publish", commentHandler.PublishComment)
			authed.POST("/delete", commentHandler.DeleteComment)
		}
	}

	// ---------- 社交关注（需登录） ----------
	socialGroup := api.Group("/social", jwt.JWTAuth(cache))
	{
		socialGroup.POST("/follow", socialHandler.Follow)
		socialGroup.POST("/unfollow", socialHandler.Unfollow)
		socialGroup.POST("/followers", socialHandler.GetAllFollowers)
		socialGroup.POST("/vloggers", socialHandler.GetAllVloggers)
		socialGroup.POST("/counts", socialHandler.GetCounts)
	}

	// ---------- 通知（需登录） ----------
	notifGroup := api.Group("/notification", jwt.JWTAuth(cache))
	{
		// 实时通知流：长连接，按当前登录账号定向推送
		notifGroup.GET("/stream", func(c *gin.Context) {
			recipientID, err := jwt.GetAccountID(c)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			hub.ServeSSE(recipientID)(c)
		})
		// 通知列表（倒序 + 游标分页）
		notifGroup.POST("/list", notificationHandler.List)
	}

	_ = cfg // 配置当前仅用于端口与 pprof，保留参数便于扩展
	return r
}

func itoa(port int) string {
	return strconv.Itoa(port)
}

// acctNameResolver 将 account 仓库适配为 notification.AccountResolver：
// 写入通知时用账号ID解析 actor 用户名做冗余，避免读取列表时 join 账号表。
type acctNameResolver struct {
	repo account.AccountRepositoryer
}

func (a acctNameResolver) UsernameByID(ctx context.Context, id uint) (string, error) {
	acc, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if acc == nil {
		return "", errors.New("account not found")
	}
	return acc.Username, nil
}
