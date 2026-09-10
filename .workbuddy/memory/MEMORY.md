# Feedi 项目长期记忆

## 时间字段单位约定（后端 ↔ 前端对齐，2026-09-09 盘点）

> 存储层：所有实体时间字段均为 MySQL DATETIME（Go `time.Time`，GORM autoCreateTime）。
> 单位只在 **JSON 序列化 / 接口参数 / Redis** 三个出口处分化，改动必须对照此表，禁止随手改单位。

### API 输出字段
| 场景 | 字段 | 单位/格式 | 证据 |
|---|---|---|---|
| feed 列表(latest/likes_count/following/popularity 共用) | `create_time` | **秒**(Unix) | feed_service.go:584 `CreateTime: video.CreateTime.Unix()` |
| video detail / publish / like my_videos(video.Video) | `create_time` | **RFC3339 字符串**(time.Time 默认) | video_entity.go json tag |
| comment 列表 | `created_at` | **RFC3339 字符串** | comment_entity.go |
| notification 列表 ListItem | `created_at` | **秒**(int64 Unix) | notification_service.go:92 / entity:44 |

### 游标 / 请求参数
| 接口 | 字段 | 单位 | 证据 |
|---|---|---|---|
| POST feed/latest | `latest_time` / 返回 `next_time` | **毫秒** | feed_handler.go `time.UnixMilli`; service `.UnixMilli()` |
| POST feed/following | `latest_time` / `next_time` | **毫秒**(2026-09-09 从秒统一为毫秒, cache key before 同步) | feed_handler.go:112; service:358 |
| POST feed/popularity | `as_of` | **秒**(随后 Truncate 到整分钟, 快照基准非翻页游标)；Redis 热榜窗口 key = `yyyyMMddHHmm` 分钟串 | feed_service.go:446-454 |
| POST feed/likes_count | —(时间无关, 用 `likes_count_before`+`id_before` 复合游标) | — | |
| POST notification/list | `before_time` / 返回 `next_before_time` | **毫秒**(2026-09-09 从秒统一为毫秒) | notification_service.go:74/96 |

### Redis / 消息内部（不对外）
| key/事件 | 单位 |
|---|---|
| `v1:feed:global_timeline` ZSET score | **毫秒**(UnixMilli)；member=videoID，裁剪保留最新 1000 条 |
| `v1:hot:video:1m:{min}` / merge 快照 | score=热度值(非时间)，窗口 key 分钟级 |
| timeline MQ 事件 `CreateTime` | **毫秒** |
| following 响应缓存 key `before` 段 | 秒 |

### 其它
- JWT：`expire_hours`/`refresh_expire_hours` 单位**小时**(config.yaml, 24/168)。
- 头像/分片文件名后缀用 UnixNano，仅命名不作 API 字段。
- 前端兼容：`fmtTime()` 按值域启发式兼容 秒/毫秒/RFC3339；`FeedVideoItem.create_time=秒`、`Video.create_time=RFC3339`、`NotificationItem.created_at=秒`、`CommentItem.created_at=RFC3339`（types.ts 各有注释）。
- 统一规则(2026-09-09 起)：「**时间游标默认毫秒**；展示字段(列表项 create_time)秒；popularity 快照基准 as_of 秒」——按接口对照上表，勿混用。
- 已知边界：时间戳游标在“同一毫秒并发发布”时翻页可能漏一条（单机低写入概率极低，暂缓复合 (time,id) 游标改造）。

## 运行/部署约定
- api 与 worker 必须同时运行；global_timeline/热度/点赞/评论事件经 outbox→RabbitMQ→worker 消费写入 Redis（只跑 api 则队列堆积、时间线为空）。
- 一键启停：仓库根 `./dev.sh start|stop|restart|status|logs`（构建并后台运行 backend/{api,worker}.exe，日志 backend/.run/logs）。WorkBuddy 会话内 nohup 会被回收，需用 run_in_background 托管 exe。
- exe 工作目录须为 backend/（默认配置 configs/config.yaml、上传目录 .run/uploads 相对它）。仓库根 api.exe/worker.exe 是旧产物。
