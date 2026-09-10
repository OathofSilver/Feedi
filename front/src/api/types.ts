// ============ 与后端 API.md 对齐的数据类型 ============

export interface Account {
  id: number
  username: string
  avatar_url?: string
  bio?: string
}

export interface AuthorBrief {
  id: number
  username: string
}

/** Feed 流条目（列表统一形态） */
export interface FeedVideoItem {
  id: number
  author: AuthorBrief
  title: string
  description?: string
  play_url: string
  cover_url: string
  /** 秒级 Unix 时间戳（feed 列表内） */
  create_time: number
  likes_count: number
  is_liked: boolean
}

/** Video 实体（detail/publish/my_videos 返回，create_time 为 RFC3339） */
export interface Video {
  id: number
  author_id: number
  username: string
  title: string
  description: string
  play_url: string
  cover_url: string
  create_time: string
  likes_count: number
  popularity: number
}

export interface CommentItem {
  id: number
  username: string
  video_id: number
  author_id: number
  content: string
  created_at: string
}

export interface FollowerItem {
  id: number
  username: string
  avatar_url?: string
  bio?: string
}

export interface NotificationItem {
  id: number
  type: 'like' | 'comment' | 'follow'
  actor_id: number
  actor_name: string
  video_id: number
  target_id: number
  content: string
  /** 毫秒级 Unix 时间戳 */
  created_at: number
}

export interface LoginResp {
  token: string
  refresh_token: string
  account_id: number
  username: string
}

export interface FeedPage {
  video_list: FeedVideoItem[]
  has_more: boolean
  // 时间游标统一毫秒（latest / following 一致）
  next_time?: number
  // likes_count composite
  next_likes_count_before?: number
  next_id_before?: number
}

export interface NotifPage {
  notifications: NotificationItem[]
  /** 下一页游标，毫秒级 Unix 时间戳，0 表示无更多 */
  next_before_time: number
}
