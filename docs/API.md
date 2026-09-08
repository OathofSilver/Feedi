# feed API 文档

> 短视频社区后端 HTTP API 契约。本文档与当前源码（`backend/internal/*`）对齐。
> 技术栈：Go + Gin + MySQL + Redis + RabbitMQ。所有业务接口挂载在 `/api/v1` 前缀下。

## 目录

- [通用约定](#通用约定)
- [鉴权机制](#鉴权机制)
- [错误码与通用响应](#错误码与通用响应)
- [账号 account](#账号-account)
- [Feed 流 feed](#feed-流-feed)
- [视频 video](#视频-video)
- [点赞 like](#点赞-like)
- [评论 comment](#评论-comment)
- [社交关注 social](#社交关注-social)
- [通知 notification](#通知-notification)
- [数据模型字段说明](#数据模型字段说明)
- [已知联调注意点](#已知联调注意点)

---

## 通用约定

- **Base URL（默认）**：`http://localhost:9000`（由 `configs/config.yaml` 的 `server.port` 决定）
- 请求/响应均为 `application/json; charset=utf-8`。
- 列表类接口用 `POST` + JSON body 传参（本项目不使用 query/path 参数传业务数据）。
- 时间字段单位不统一，**见各接口说明与[已知联调注意点](#已知联调注意点)**。
- 健康检查：`GET /healthz` → `{"status":"ok"}`。
- 静态资源：`/static/avatars/{file}`（头像）。注意：视频/封面上传返回的 URL 当前未注册静态路由，见[已知联调注意点](#已知联调注意点)。

### 鉴权等级

| 等级 | 说明 | 适用接口 |
|------|------|----------|
| 公开 | 无需登录 | 注册、登录、刷新、账号查询、Feed 流(可附token)、视频详情、评论列表 |
| 登录（JWTAuth） | 必须带有效 access token | 点赞、评论发布/删除、关注、视频发布/上传/删除、账号改名等 |
| 软登录（SoftJWTAuth） | 带 token 则校验并注入身份，不带则视为游客 | Feed 流、视频详情 |

---

## 鉴权机制

### Access Token（JWT）

- Header：`Authorization: Bearer <access_token>`
- 签发算法：`HS256`。默认有效期 `24h`（`jwt.expire_hours`）。
- 服务端做**单会话**控制：登录后 token 写入 Redis，登出/改名/重置密码会使其失效；新登录会顶掉旧 token。Redis 不可用时降级为仅 JWT 验签。
- Claims：`account_id`、`username`、`exp`、`iat`、`nbf`。

### Refresh Token（opaque，存 Redis）

- 默认有效期 `168h`（7 天，`jwt.refresh_expire_hours`）。
- 仅用于 `POST /account/refresh` 换发 access token，不参与其他接口鉴权。
- 重新登录会覆盖旧 refresh token。

### 登录后如何拿当前用户

- 鉴权中间件会把 `accountID` / `username` 注入上下文。对登录接口，业务用当前登录者的 ID 操作，**不需要传自己的 ID**（例如点赞、评论、关注）。
- Feed 流在携带 token 时返回 `is_liked` 等个性化状态；不携带时 `is_liked` 一律为 `false`。

---

## 错误码与通用响应

### HTTP 状态码语义

| 状态码 | 含义 |
|--------|------|
| 200 | 成功 |
| 400 | 参数错误 / 业务校验失败 |
| 401 | 未认证 / token 无效或已吊销 |
| 404 | 资源不存在（gorm.ErrRecordNotFound） |
| 500 | 服务内部错误 |

> ⚠️ 注意：不少 handler 直接把业务错误返回 `500`（如重复点赞、重复关注返回 500）。本文档按**实际实现**标注，联调请以后端实际返回为准。

### 统一错误体

```json
{ "error": "<错误描述>" }
```

成功时部分接口返回：

```json
{ "message": "<提示文本>" }
```

---

## 账号 account

### POST /api/v1/account/register 注册

公开。请求：

```json
{ "username": "alice", "password": "secret123" }
```

响应 `200`：

```json
{ "message": "account created" }
```

### POST /api/v1/account/login 登录

公开。请求：

```json
{ "username": "alice", "password": "secret123" }
```

响应 `200`：

```json
{
  "token": "<access_token>",
  "refresh_token": "<refresh_token>",
  "account_id": 1,
  "username": "alice"
}
```

### POST /api/v1/account/refresh 刷新 access token

公开。请求：

```json
{ "refresh_token": "<refresh_token>" }
```

响应 `200`（成功签发新 token）：

```json
{
  "token": "<new_access_token>",
  "account_id": 1,
  "username": "alice"
}
```

> refresh 失败返回 `401 {"error":"invalid refresh token"}`。

### POST /api/v1/account/info 按 ID 查询账号

公开。请求：

```json
{ "id": 1 }
```

响应 `200`（字段同 `account` 实体，密码不返回）：

```json
{
  "id": 1,
  "username": "alice",
  "avatar_url": "/static/avatars/avatar_1_1700000000.jpg",
  "bio": "hello"
}
```

### POST /api/v1/account/username 按用户名查询账号

公开。请求：

```json
{ "username": "alice" }
```

响应 `200`：

```json
{ "id": 1, "username": "alice" }
```

### POST /api/v1/account/logout 登出（需登录）

请求体：空 `{}`。

响应 `200`：

```json
{ "message": "account logged out" }
```

### POST /api/v1/account/rename 改名（需登录）

请求：

```json
{ "new_username": "alice_new" }
```

响应 `200`：

```json
{ "token": "<new_access_token>" }
```

> 改名会换发新 token（并使旧 token 失效），前端需用返回的新 token 更新本地缓存。

### POST /api/v1/account/password 修改密码（需登录）

请求：

```json
{ "username": "alice", "old_password": "old", "new_password": "new" }
```

响应 `200`：

```json
{ "message": "successfully password changed" }
```

### POST /api/v1/account/profile 更新个人资料（需登录）

请求（`avatar_url` / `bio` 至少填一项，不填的字段不更新）：

```json
{ "avatar_url": "/static/avatars/avatar_1_x.jpg", "bio": "about me" }
```

响应 `200`：

```json
{ "message": "个人资料更新成功" }
```

> 两项都为空 → `400`。

### POST /api/v1/account/avatar 上传头像（需登录，multipart）

- 字段名：`file`
- 大小限制：≤ 5MB
- 允许扩展名：`jpg / jpeg / png / webp / gif`
- 请求体：`multipart/form-data`

响应 `200`：

```json
{ "url": "/static/avatars/avatar_1_1700000000000.jpg", "message": "头像上传成功" }
```

---

## Feed 流 feed

游客可访问；携带 token 时注入 `viewerAccountID` 并返回 `is_liked` 个性化状态。

所有 feed 列表单页上限均为 `50`，默认 `10`；`limit <= 0 或 > 50` 时归为 `10`。

**Feed 条目结构 `FeedVideoItem`**（各列表共用的返回元素）：

```json
{
  "id": 1,
  "author": { "id": 1, "username": "alice" },
  "title": "标题",
  "description": "简介",
  "play_url": "http://...",
  "cover_url": "http://...",
  "create_time": 1700000000,
  "likes_count": 12,
  "is_liked": false
}
```

> 各列表内 `create_time` 为 **Unix 秒**（秒级）。

### POST /api/v1/feed/latest 最新视频（时间线，冷热分离+游标）

请求：

```json
{ "limit": 20, "latest_time": 1700000000000 }
```

- `latest_time`：游标，**Unix 毫秒**。为 `0` 表示第一页（最新）。下一页传上一响应 `next_time`。

响应 `200`：

```json
{
  "video_list": [ { "id": 1, "...": "" } ],
  "next_time": 1699999999000,
  "has_more": true
}
```

> 翻页：`next_time` 填回 `latest_time`。数据来自 Redis 全局时间线 ZSet（保留最新约 1000 条）+ MySQL 冷数据拼接。

### POST /api/v1/feed/likes_count 按点赞数热门

请求（第一页不带游标；翻页时两游标必须成对传）：

```json
{
  "limit": 20,
  "likes_count_before": 10,
  "id_before": 33
}
```

响应 `200`：

```json
{
  "video_list": [ ... ],
  "next_likes_count_before": 8,
  "next_id_before": 20,
  "has_more": true
}
```

> 翻页：把响应的 `next_likes_count_before` / `next_id_before` 成对回填。二者要么都传，要么都不传，否则 `400`。

### POST /api/v1/feed/following 关注流（关注的人发布的视频）

请求：

```json
{ "limit": 20, "latest_time": 1700000000 }
```

- `latest_time`：游标，**Unix 秒**。`0` 表示第一页，翻页传上页 `next_time`。

响应 `200`：

```json
{
  "video_list": [ ... ],
  "next_time": 1699999999,
  "has_more": true
}
```

> ⚠️ 与 `/feed/latest` 不同，这里的 `latest_time`/`next_time` 是**秒**不是毫秒（见注意点）。

### POST /api/v1/feed/popularity 热度推荐榜（近 1 小时累计热度）

请求（第一页：`offset=0, as_of=0`）：

```json
{
  "limit": 20,
  "as_of": 0,
  "offset": 0,
  "latest_popularity": 0,
  "latest_before": "0001-01-01T00:00:00Z",
  "latest_id_before": 0
}
```

字段说明：
- `as_of`：榜单快照时间（Unix 秒，通常由上一响应返回并复用，保证翻页榜单不变）；`0` 用当前时刻。
- `offset`：本次跳过的条数（Redis 快照分页用）。
- `latest_popularity` / `latest_before` / `latest_id_before`：MySQL 兜底复合游标，**三者需成对携带**（仅当落到 DB 兜底分页时使用）。

响应 `200`：

```json
{
  "video_list": [ ... ],
  "as_of": 1700000400,
  "next_offset": 20,
  "has_more": true,
  "next_latest_popularity": 88,
  "next_latest_before": "2023-11-15T10:00:00Z",
  "next_latest_id_before": 5
}
```

- 命中 Redis：翻页用 `as_of`（不变）+ `next_offset`。
- 未命中/降级到 MySQL：`as_of=0, next_offset=0`，用 `next_latest_*` 复合游标翻页。

### POST /api/v1/feed/tag 按标签查询

请求：

```json
{ "tag_name": "旅行", "limit": 20 }
```

响应 `200`：

```json
{
  "video_list": [ ... ]
}
```

---

## 视频 video

### POST /api/v1/video/publish 发布视频（需登录）

请求：

```json
{
  "title": "我的视频",
  "description": "简介",
  "play_url": "http://.../video.mp4",
  "cover_url": "http://.../cover.jpg"
}
```

响应 `200`（返回完整 Video 实体）：

```json
{
  "id": 1,
  "author_id": 1,
  "username": "alice",
  "title": "我的视频",
  "description": "简介",
  "play_url": "http://...",
  "cover_url": "http://...",
  "create_time": "2026-09-07T10:00:00Z",
  "likes_count": 0,
  "popularity": 0
}
```

> 发布会写 outbox 事务消息，异步投递到全局时间线 / 标签索引。

### POST /api/v1/video/upload 上传视频文件（需登录，multipart）

- 字段名：`file`；仅 `.mp4`；≤ 200MB。
- 请求体：`multipart/form-data`。

响应 `200`：

```json
{ "url": "http://localhost:9000/static/videos/1/20260907/ab12.mp4", "play_url": "http://localhost:9000/static/videos/1/20260907/ab12.mp4" }
```

### POST /api/v1/video/cover 上传封面（需登录，multipart）

- 字段名：`file`；允许 `jpg/jpeg/png/webp`；≤ 10MB。

响应 `200`：

```json
{ "url": "http://localhost:9000/static/covers/1/20260907/ab12.jpg", "cover_url": "http://localhost:9000/static/covers/1/20260907/ab12.jpg" }
```

> ⚠️ 上传返回的视频/封面 URL 走 `/static/videos|covers/...`，但服务当前**只注册了 `/static/avatars`** 静态路由，这两个前缀会 404（见注意点）。

### POST /api/v1/video/detail 视频详情（软登录）

请求：

```json
{ "id": 1 }
```

响应 `200`（Video 实体，`create_time` 为 RFC3339）：

```json
{
  "id": 1, "author_id": 1, "username": "alice",
  "title": "x", "description": "y",
  "play_url": "http://...", "cover_url": "http://...",
  "create_time": "2026-09-07T10:00:00Z",
  "likes_count": 5, "popularity": 9
}
```

### POST /api/v1/video/delete 删除视频（需登录，仅作者本人）

请求：

```json
{ "id": 1 }
```

响应 `200`：

```json
{ "message": "video deleted" }
```

### POST /api/v1/video/likes_count/update 更新点赞数（需登录）

> ⚠️ 该接口按代码为 **SET 语义但使用增量 SQL**，语义不明确，一般不建议业务调用（计数由点赞/评论链路自动维护）。文档保留以对齐实现。

请求：

```json
{ "id": 1, "likes_count": 5 }
```

响应 `200`：

```json
{ "message": "likes count updated" }
```

---

## 点赞 like

点赞记录同步落库；计数与热度通过 MQ 异步、Redis 幂等去重后更新。

### POST /api/v1/like/action 点赞（需登录）

请求：

```json
{ "video_id": 1 }
```

响应 `200`：

```json
{ "message": "like success" }
```

### POST /api/v1/like/unlike 取消点赞（需登录）

请求：

```json
{ "video_id": 1 }
```

响应 `200`：

```json
{ "message": "unlike success" }
```

### POST /api/v1/like/is_liked 是否已点赞（需登录）

请求：

```json
{ "video_id": 1 }
```

响应 `200`：

```json
{ "is_liked": true }
```

### POST /api/v1/like/my_videos 我点赞过的视频列表（需登录）

请求体：空 `{}`。

响应 `200`（Video 实体数组，`create_time` 为 RFC3339）：

```json
[ { "id": 1, "...": "..." } ]
```

> 无数据时返回 `[]`。

---

## 评论 comment

### POST /api/v1/comment/publish 发布评论（需登录）

请求：

```json
{ "video_id": 1, "content": "好看！" }
```

响应 `200`：

```json
{ "message": "comment published successfully" }
```

### POST /api/v1/comment/delete 删除评论（需登录，仅本人）

请求：

```json
{ "comment_id": 3 }
```

响应 `200`：

```json
{ "message": "comment deleted successfully" }
```

### POST /api/v1/comment/list 获取某视频评论列表（公开）

请求：

```json
{ "video_id": 1 }
```

响应 `200`（Comment 数组，`created_at` 为 RFC3339）：

```json
[
  {
    "id": 1, "username": "alice", "video_id": 1, "author_id": 1,
    "content": "好看！", "created_at": "2026-09-07T10:00:00Z"
  }
]
```

---

## 社交关注 social

### POST /api/v1/social/follow 关注（需登录）

请求：

```json
{ "vlogger_id": 2 }
```

> 关注者 = 当前登录者。

响应 `200`：

```json
{ "message": "followed" }
```

### POST /api/v1/social/unfollow 取消关注（需登录）

请求：

```json
{ "vlogger_id": 2 }
```

响应 `200`：

```json
{ "message": "unfollowed" }
```

### POST /api/v1/social/followers 我的粉丝列表（需登录）

请求：空 `{}`（`vlogger_id=0` 时用当前登录者）。

> 也可显式传 `{ "vlogger_id": 5 }` 查他人粉丝。

响应 `200`：

```json
{
  "followers": [ { "id": 2, "username": "bob", "avatar_url": "...", "bio": "..." } ],
  "follower_count": 1
}
```

### POST /api/v1/social/vloggers 我关注的人列表（需登录）

请求：空 `{}`（`follower_id=0` 时用当前登录者）。

> 也可显式传 `{ "follower_id": 5 }` 查他人关注列表。

响应 `200`：

```json
{
  "vloggers": [ { "id": 1, "username": "alice" } ],
  "vlogger_count": 1
}
```

### POST /api/v1/social/counts 我/自己的关注数与粉丝数（需登录）

请求体：空 `{}`。

响应 `200`：

```json
{ "follower_count": 3, "vlogger_count": 2 }
```

---

## 通知 notification

通知为**同步落库**（列表权威源）+ MQ 驱动实时 SSE 推送。触发场景：有人点赞/评论/关注了你。

### POST /api/v1/notification/list 通知列表（需登录，游标分页）

请求：

```json
{ "before_time": 0, "limit": 20 }
```

- `before_time`：上一页最后一条通知 `created_at` 的 **Unix 秒**；`0` 表示第一页。
- `limit`：默认 `20`，服务端上限 `100`（repo 层钳制）。

响应 `200`：

```json
{
  "notifications": [
    {
      "id": 1,
      "type": "like",
      "actor_id": 2,
      "actor_name": "bob",
      "video_id": 10,
      "target_id": 0,
      "content": "",
      "created_at": 1700000000
    }
  ],
  "next_before_time": 1699999900
}
```

字段说明：

| 字段 | 说明 |
|------|------|
| `type` | `like`（赞）/ `comment`（评论）/ `follow`（关注） |
| `video_id` | like/comment 指向的视频；follow 为 0 |
| `target_id` | follow 的被关注者(=recipient)；like/comment 为 0 |
| `content` | comment 内容快照；like/follow 为空 |
| `created_at` | Unix 秒 |

翻页：`next_before_time` 回填 `before_time`；为 `0` 表示无更多。

### GET /api/v1/notification/stream 实时通知流 SSE（需登录）

- Header 鉴权同其他登录接口。
- 打开后保持长连接，服务端 `Content-Type: text/event-stream`，每 25s 发 `: ping` 心跳防代理超时。

收到的事件体（`data:` 行，JSON）：

```json
{ "event_id": "<消息去重ID>", "notification_id": 1, "recipient_id": 2 }
```

```text
data: {"event_id":"...","notification_id":1,"recipient_id":2}

```

> 事件只含通知主键，**不含内容**。前端收到后建议调 `POST /notification/list` 拉取/刷新最新列表展示。

---

## 数据模型字段说明

### account.Account（对外输出不含密码）

| 字段 | JSON | 说明 |
|------|------|------|
| id | `id` | 主键 |
| username | `username` | 用户名（唯一） |
| avatar_url | `avatar_url` | 头像（可空） |
| bio | `bio` | 简介（可空） |

### video.Video

| 字段 | JSON | 说明 |
|------|------|------|
| id | `id` | 主键 |
| author_id | `author_id` | 作者账号 ID |
| username | `username` | 作者用户名（冗余） |
| title / description | `title` / `description` | 标题 / 简介 |
| play_url / cover_url | `play_url` / `cover_url` | 播放 / 封面 URL |
| create_time | `create_time` | **RFC3339**（实体输出） |
| likes_count | `likes_count` | 点赞总数 |
| popularity | `popularity` | 热度分值 |

### comment.Comment

| 字段 | JSON | 说明 |
|------|------|------|
| id | `id` | 主键 |
| username | `username` | 作者名（冗余） |
| video_id | `video_id` | 所属视频 |
| author_id | `author_id` | 作者账号 ID |
| content | `content` | 内容 |
| created_at | `created_at` | **RFC3339** |

---

## 已知联调注意点

> 以下为**当前实现**中会影响前端联调的细节，均已按源码确认。

1. **`latest_time` 单位不统一**
   - `/feed/latest`：`latest_time`/`next_time` 为 **Unix 毫秒**。
   - `/feed/following`：`latest_time`/`next_time` 为 **Unix 秒**。
   - 混用会导致游标错位，请按接口各自取回其响应值回填。

2. **时间输出格式不一致**
   - Feed 列表条目 `create_time` = Unix **秒**。
   - Video / Comment 实体 `create_time` / `created_at` = **RFC3339** 字符串。

3. **视频/封面上传 URL 暂不可访问**：上传返回 `/static/videos|/static/covers/...`，但服务仅注册了 `/static/avatars` 静态目录，`/static/videos` 与 `/static/covers` 当前会返回 404。发布时需自备可访问的 CDN/对象存储 URL。

4. **重复操作 / 部分业务错误返回 500**：重复点赞（`已点赞`）、未点赞取消、重复关注等目前走 `500`，而非 `4xx`。前端请勿只按 `!=200` 判失败文案，建议透传 `error` 字段。

5. **通知流事件无内容**：SSE `data` 仅含 `notification_id`，展示需二次拉取 `/notification/list`。

6. **关注流新视频有时效延迟**：关注流结果做了最长 24h 的响应缓存，作者发布新视频后，关注列表可能在缓存过期后才出现（详见代码 review，暂未修复）。
