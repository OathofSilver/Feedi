import { ref } from 'vue'
import type { FeedVideoItem, FeedPage } from '@/api/types'

/** loader 接收一个序列化游标对象，返回新页面。cursor 由外部维护快照对象。 */
export type FeedLoader = (cursorObj: Record<string, any>) => Promise<FeedPage>

/**
 * 沉浸式 Feed 分页加载器。
 * 游标类型可因接口而异，统一用 Record<string, any> 携带，由外部 loader 自解析。
 * pageSize：每页条数（后端上限 50，默认 6 保证竖屏顺畅）。
 *
 * 语义：
 * - refresh()：清空并用第一页替换列表（切换 tab / 下拉刷新场景）；
 * - loadMore()：基于当前游标追加下一页；
 * - 内部用自增 seq 丢弃过期响应，避免 refresh 与在途 loadMore 交错污染列表。
 */
export function useInfiniteFeed(loader: FeedLoader, pageSize = 6) {
  const list = ref<FeedVideoItem[]>([])
  const loading = ref(false)
  const done = ref(false)
  const error = ref('')

  let cursorObj: Record<string, any> = {}
  let seq = 0

  /** 首次 / 切换源时加载：清空旧列表并以新首页替换 */
  async function refresh() {
    const my = ++seq
    loading.value = true
    error.value = ''
    cursorObj = {}
    try {
      const res = await loader(cursorObj)
      if (my !== seq) return // 已被更新的 refresh 取代
      advanceCursor(res)
      list.value = res.video_list || []
      done.value = !res.has_more || (res.video_list || []).length === 0
    } catch (e) {
      if (my !== seq) return
      // 刷新失败：保留已有内容，仅提示错误（空列表时由调用方展示错误态）
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      if (my === seq) loading.value = false
    }
  }

  async function loadMore() {
    if (loading.value || done.value) return
    const my = seq
    loading.value = true
    try {
      const res = await loader(cursorObj)
      if (my !== seq) return // refresh 已重置数据，丢弃过期结果
      advanceCursor(res)
      append(res)
    } catch (e) {
      if (my !== seq) return
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      if (my === seq) loading.value = false
    }
  }

  /** 从响应推进游标快照 */
  function advanceCursor(res: FeedPage) {
    const next: Record<string, any> = {}
    if (typeof res.next_time === 'number') next.next_time = res.next_time
    if (typeof res.next_likes_count_before === 'number') next.next_likes_count_before = res.next_likes_count_before
    if (typeof res.next_id_before === 'number') next.next_id_before = res.next_id_before
    if (typeof (res as any).next_offset === 'number') next.next_offset = (res as any).next_offset
    if (typeof (res as any).as_of === 'number' && (res as any).as_of !== 0) next.as_of = (res as any).as_of
    cursorObj = next
  }

  function append(res: FeedPage) {
    const seen = new Set(list.value.map((i) => i.id))
    const fresh = (res.video_list || []).filter((i) => !seen.has(i.id))
    list.value.push(...fresh)
    if (!res.has_more || (res.video_list || []).length === 0) done.value = true
  }

  function reset() {
    seq++
    list.value = []
    cursorObj = {}
    done.value = false
    loading.value = false
    error.value = ''
  }

  return { list, loading, done, error, refresh, loadMore, reset }
}
