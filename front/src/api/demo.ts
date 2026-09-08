// ============ 演示数据：真实后端 feed 为空/出错时兜底，保证页面始终可演示 ============
import type { FeedVideoItem } from '@/api/types'

const topics = [
  '夏日city walk，转角遇见晚霞',
  '这杯特调咖啡治愈了整个下午',
  '铲屎官的崩溃瞬间，过于真实',
  '跟着镜头去看海吧，蓝到不真实',
  '厨房小白也能做的三分钟早餐',
  '深夜食堂，一个人的治愈系晚餐',
  '公路旅行第7天，风景美到词穷',
  '这只小奶猫把我的心融化了',
  '健身房打卡Day 30，变化肉眼可见',
  '探店｜藏在巷子里的宝藏书店',
  '露营日记：星空下的篝火',
  '雨天窗边的白噪音，适合放空',
]

const authors = [
  '林深见鹿',
  '橘子汽水',
  '阿茶不加糖',
  '山与海',
  '野生摄影师KK',
  '喵喵指挥家',
  '碳水快乐指南',
  '小城青年',
  '风里有诗',
  '晚风便利店',
]

function mulberry32(a: number) {
  return function () {
    a |= 0
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export function makeDemoItems(count = 14): FeedVideoItem[] {
  const rand = mulberry32(20260907)
  const now = Math.floor(Date.now() / 1000)
  const items: FeedVideoItem[] = []
  for (let i = 0; i < count; i++) {
    const authorId = Math.floor(rand() * 900) + 100
    const username = authors[Math.floor(rand() * authors.length)]
    const title = topics[Math.floor(rand() * topics.length)]
    const likes = Math.floor(rand() * 90000) + 500
    items.push({
      id: 9_000_000 + i,
      author: { id: authorId, username: `${username}${i % 3 === 0 ? '' : Math.floor(rand() * 99 + 1)}` },
      title,
      description: title,
      play_url: '', // 无真实媒体，触发演示占位层
      cover_url: '',
      create_time: now - i * 3600 * (Math.floor(rand() * 20) + 2),
      likes_count: likes,
      is_liked: false,
    })
  }
  return items
}
