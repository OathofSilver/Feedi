import { ref } from 'vue'
import type { FeedVideoItem, FeedPage } from '@/api/types'

/** loader 接收一个序列化游标对象，返回新页面。cursor 由外部维护快照对象。 */
export type FeedLoader = (cursorObj: Record<string, any>) => Promise<FeedPage>

/**
 * 沉浸式 Feed 分页加载器。
 * 游标类型可因接口而异，统一用 Record<string, any> 携带，由外部 loader 自解析。
 * pageSize：每页条数（后端上限 50，默认 6 保证竖屏顺畅）。
 */
export function useInfiniteFeed(loader: FeedLoader, pageSize = 6) {
  const list = ref<FeedVideoItem[]>([])
  const loading = ref(false)
  const done = ref(false)
  const error = ref('')

  let cursorObj: Record<string, any> = {}

  /** 首次 / 切换源时加载 */
  async function refresh() {
    loading.value = true
    error.value = ''
    cursorObj = {}
    try {
      const res = await loader(cursorObj)
      advanceCursor(res)
      append(res)
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      loading.value = false
    }
  }

  async function loadMore() {
    if (loading.value || done.value) return
    loading.value = true
    try {
      const res = await loader(cursorObj)
      advanceCursor(res)
      append(res)
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      loading.value = false
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
    if (!res.has_more || (res.video_list && res.video_list.length === 0)) done.value = true
  }

  function reset() {
    list.value = []
    cursorObj = {}
    done.value = false
    loading.value = false
    error.value = ''
  }

  return { list, loading, done, error, refresh, loadMore, reset }
}
