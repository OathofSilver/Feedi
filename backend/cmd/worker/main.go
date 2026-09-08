// Worker 进程入口：负责异步消息消费。
// 启动流程：加载配置 -> 连接 MySQL/Redis/RabbitMQ -> 组装仓库与消费者
// -> errgroup 并发运行全部消费循环 -> 收到退出信号后优雅关闭。
//
// 启动方式（任选其一）：
//
//	go run ./cmd/worker            # 在 backend 目录下
//	go run ./backend/cmd/worker    # 在项目根目录下
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"feed/backend/internal/config"
	"feed/backend/internal/utils/mysql"
	"feed/backend/internal/utils/rabbitmq"
	rediscache "feed/backend/internal/utils/redis"
	"feed/backend/internal/video"
	"feed/backend/internal/worker/consumer"

	"golang.org/x/sync/errgroup"
)

// 队列名称，与 internal/worker/producer 中各生产者的队列声明保持一致
const (
	likeQueue       = "like.events"             // 点赞/取消点赞事件队列
	commentQueue    = "comment.events"          // 评论发布/删除事件队列
	popularityQueue = "video.popularity.events" // 视频热度变更事件队列
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker 进程异常退出", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// ---------- 配置 ----------
	cfg, err := config.LoadDefault()
	if err != nil {
		return err
	}

	// ---------- 优雅退出：信号 -> ctx 取消 ----------
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	g, ctx := errgroup.WithContext(ctx)

	// ---------- 基础设施：MySQL / Redis / RabbitMQ ----------
	db, err := mysql.NewDB(cfg.Database)
	if err != nil {
		return err
	}
	// 建/补全数据表（NewDB 内已 ensure 库）。api 与 worker 都可能独立启动，
	// 两端都迁移以保证单独部署也能自建表；建议 api 先于 worker 启动以规避并发 DDL 竞态。
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

	// ---------- 仓库层（消费者落地业务数据用） ----------
	videoRepo := video.NewVideoRepository(db)

	// 点赞消费者：点赞记录已由 API 侧同步落库，这里只做计数/热度增量更新，
	// 幂等依赖 Redis 事件去重，无需点赞仓库
	likeCh, err := baseMQ.NewChannel()
	if err != nil {
		return err
	}
	defer func() {
		if err := likeCh.Close(); err != nil {
			slog.Warn("关闭 like channel 失败", "err", err)
		}
	}()
	likeWorker := consumer.NewLikeWorker(likeCh, videoRepo, cache, likeQueue)

	// 评论消费者：评论记录已由 API 侧同步落库，这里只做热度 DB 增量副作用，
	// 幂等依赖 Redis 事件去重；删除事件的 video_id 由生产者冗余携带，无需评论仓库反查
	commentCh, err := baseMQ.NewChannel()
	if err != nil {
		return err
	}
	defer func() {
		if err := commentCh.Close(); err != nil {
			slog.Warn("关闭 comment channel 失败", "err", err)
		}
	}()
	commentWorker := consumer.NewCommentWorker(commentCh, videoRepo, cache, commentQueue)

	// 热度消费者：更新 Redis 分钟热度窗口并清理视频详情缓存
	popularityCh, err := baseMQ.NewChannel()
	if err != nil {
		return err
	}
	defer func() {
		if err := popularityCh.Close(); err != nil {
			slog.Warn("关闭 popularity channel 失败", "err", err)
		}
	}()
	popularityWorker := consumer.NewPopularityWorker(popularityCh, cache, popularityQueue)

	// 全局推荐时间线消费者：视频写入全局时间线 ZSet（构造时自动声明拓扑并设置 Qos）
	globalTimeline, err := consumer.NewGlobalTimelineConsumer(baseMQ, cache)
	if err != nil {
		return err
	}
	defer func() {
		if err := globalTimeline.Close(); err != nil {
			slog.Warn("关闭 global timeline consumer 失败", "err", err)
		}
	}()

	// 并发运行全部消费循环。
	g.Go(func() error {
		return likeWorker.Run(ctx)
	})
	g.Go(func() error {
		return commentWorker.Run(ctx)
	})
	g.Go(func() error {
		return popularityWorker.Run(ctx)
	})
	g.Go(func() error {
		return globalTimeline.Run(ctx)
	})

	slog.Info("worker 进程已启动",
		"consumers", []string{"like", "comment", "popularity", "global_timeline"})

	// 任一消费循环退出（ctx 取消或出错）即开始收尾
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	slog.Info("worker 进程已退出")
	return nil
}
