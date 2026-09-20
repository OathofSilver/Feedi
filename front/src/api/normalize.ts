// 时间/资源归一化工具

/**
 * 智能时间格式化：兼容 Unix 秒 / Unix 毫秒 / RFC3339 字符串。
 * 返回「刚刚 / x分钟前 / x小时前 / MM-DD HH:mm / yyyy年」。
 */
export function fmtTime(input: number | string | undefined | null): string {
  if (input === undefined || input === null || input === '') return ''
  let ts: number
  if (typeof input === 'string') {
    ts = new Date(input).getTime()
    if (Number.isNaN(ts)) return input
  } else {
    // 秒(10位) vs 毫秒(13位)
    ts = input < 1e12 ? input * 1000 : input
  }
  const diff = Date.now() - ts
  const sec = Math.floor(diff / 1000)
  if (sec < 0) return '刚刚'
  if (sec < 60) return '刚刚'
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}分钟前`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}小时前`
  const day = Math.floor(hr / 24)
  if (day < 7) return `${day}天前`

  const d = new Date(ts)
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  if (d.getFullYear() === now.getFullYear()) {
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** 千分位 / 缩写计数 */
export function fmtCount(n: number): string {
  if (n >= 10000) {
    const w = n / 10000
    return (w >= 100 ? Math.round(w) : Math.floor(w * 10) / 10) + 'w'
  }
  return String(n)
}

// 内置演示封面（渐变色块，无真实媒体时兜底，避免白屏）
const DEMO_GRADIENTS = [
  'linear-gradient(135deg,#fe2c55 0%,#fe5f3c 100%)',
  'linear-gradient(135deg,#25f4ee 0%,#1c8fff 100%)',
  'linear-gradient(135deg,#7b2cfe 0%,#fe2c8a 100%)',
  'linear-gradient(135deg,#ffd200 0%,#ff7a2c 100%)',
]

/** 根据 id 取稳定的演示渐变（用于封面占位/播放失败兜底） */
export function demoGradient(seed: number): string {
  const i = Math.abs(seed) % DEMO_GRADIENTS.length
  return DEMO_GRADIENTS[i]
}
