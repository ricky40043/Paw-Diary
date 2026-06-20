<template>
  <div class="admin">
    <!-- 登入 -->
    <div v-if="!authed" class="login-box">
      <h1>🐾 抓抓日記 後台</h1>
      <p class="hint">請輸入管理 Token</p>
      <input v-model="token" type="password" placeholder="ADMIN_TOKEN" @keyup.enter="login" />
      <button @click="login" :disabled="loading">{{ loading ? '驗證中…' : '進入後台' }}</button>
      <p v-if="error" class="err">{{ error }}</p>
    </div>

    <!-- 後台 -->
    <div v-else class="dash">
      <header>
        <h1>🐾 抓抓日記 後台</h1>
        <button class="logout" @click="logout">登出</button>
      </header>

      <!-- 統計 -->
      <section class="stats" v-if="stats">
        <div class="cards">
          <div class="card"><span class="num">{{ stats.total }}</span><span class="lbl">總任務</span></div>
          <div class="card ok"><span class="num">{{ stats.completed }}</span><span class="lbl">成功</span></div>
          <div class="card bad"><span class="num">{{ stats.failed }}</span><span class="lbl">失敗</span></div>
          <div class="card"><span class="num">{{ successRate }}%</span><span class="lbl">成功率</span></div>
          <div class="card"><span class="num">{{ (stats.avg_total_ms/1000).toFixed(0) }}s</span><span class="lbl">平均生成</span></div>
          <div class="card"><span class="num">{{ stats.avg_videos.toFixed(1) }}</span><span class="lbl">平均影片數</span></div>
        </div>
        <div class="charts">
          <div class="chart"><h3>風格分布</h3><BarList :data="stats.by_mode" /></div>
          <div class="chart"><h3>互動類型分布</h3><BarList :data="stats.by_interaction" /></div>
          <div class="chart"><h3>情緒分布</h3><BarList :data="stats.by_emotion" /></div>
        </div>
      </section>

      <!-- 任務列表 -->
      <section class="tasks">
        <h2>任務列表（{{ totalTasks }}）</h2>
        <table>
          <thead>
            <tr><th>時間</th><th>狗狗</th><th>帳號</th><th>風格</th><th>影片</th><th>狀態</th><th>耗時</th><th></th></tr>
          </thead>
          <tbody>
            <tr v-for="t in tasks" :key="t.id" @click="openDetail(t.id)" class="row">
              <td>{{ fmtDate(t.created_at) }}</td>
              <td>{{ t.dog_name || '—' }}</td>
              <td>{{ t.username ? '👤 ' + t.username : '匿名' }}</td>
              <td>{{ modeLabel(t.story_mode) }}</td>
              <td>{{ t.video_count }}</td>
              <td><span :class="['badge', t.status]">{{ statusLabel(t.status) }}</span></td>
              <td>{{ t.total_ms ? (t.total_ms/1000).toFixed(0)+'s' : '—' }}</td>
              <td>›</td>
            </tr>
            <tr v-if="!tasks.length"><td colspan="8" class="empty">尚無任務資料</td></tr>
          </tbody>
        </table>
      </section>
    </div>

    <!-- 詳情 -->
    <div v-if="detail" class="modal" @click.self="detail = null">
      <div class="modal-box">
        <button class="close" @click="detail = null">✕</button>
        <h2>{{ detail.task.dog_name }}的任務</h2>
        <p class="meta">
          <span :class="['badge', detail.task.status]">{{ statusLabel(detail.task.status) }}</span>
          {{ fmtDate(detail.task.created_at) }} · {{ modeLabel(detail.task.story_mode) }} · {{ detail.task.video_count }} 支影片
          · {{ detail.task.is_anonymous ? '👤 匿名' : '👤 ' + detail.task.username }}
        </p>
        <p v-if="detail.task.error" class="err">錯誤：{{ detail.task.error }}</p>

        <div class="kv">
          <div><b>主人留言</b><p>{{ detail.task.owner_message || '—' }}</p></div>
          <div><b>劇本標題</b><p>{{ detail.task.story_title || '—' }}</p></div>
          <div><b>狗狗回話</b><p>{{ detail.task.dog_response || '—' }}</p></div>
          <div><b>模型</b><p>視覺：{{ detail.task.vision_model }}<br>文字：{{ detail.task.text_model }}</p></div>
          <div><b>耗時</b><p>分析 {{ ms(detail.task.analysis_ms) }}、劇本 {{ ms(detail.task.story_ms) }}、合成 {{ ms(detail.task.composite_ms) }}、總 {{ ms(detail.task.total_ms) }}</p></div>
        </div>

        <a v-if="detail.task.final_video" :href="storageUrl(detail.task.final_video)" target="_blank" class="watch">▶ 看最終影片</a>

        <h3>每支影片分析 vs 最終旁白</h3>
        <div class="vid" v-for="v in sortedVideos" :key="v.video_index">
          <div class="vid-head">
            <span class="vidx">#{{ v.video_index }}</span>
            <span class="pos" v-if="v.playback_position >= 0">播放第 {{ v.playback_position }} 段</span>
            <span class="pos unused" v-else>未使用</span>
            <span class="fname">{{ v.original_name }}</span>
            <span class="sz">{{ (v.size_bytes/1048576).toFixed(1) }}MB</span>
          </div>
          <div class="vid-body">
            <div class="col">
              <b>👁 分析（畫面看到的）</b>
              <p v-if="v.short_caption === '影片分析'" class="failed">⚠️ 這支視覺分析失敗，以下為預設值（非真實畫面分析）</p>
              <p class="caption">{{ v.short_caption || '—' }}</p>
              <p class="tags">
                <span class="tag">互動：{{ interLabel(v.interaction_type) }}</span>
                <span class="tag">情緒：{{ v.emotion || '—' }}</span>
                <span class="tag">{{ v.has_human ? '有人' : '只有狗' }}</span>
                <span class="tag">剪 {{ v.clip_start.toFixed(0) }}–{{ v.clip_end.toFixed(0) }}s</span>
              </p>
            </div>
            <div class="col">
              <b>✍️ 最終旁白</b>
              <p class="narration">{{ v.narration || '（這支沒被寫進劇本）' }}</p>
            </div>
          </div>
        </div>

        <div class="danger-zone">
          <button class="del-btn" @click="deleteTask(detail.task.id)" :disabled="deleting">
            {{ deleting ? '刪除中…' : '🗑 刪除這個任務（含最終影片）' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, h } from 'vue'
import axios from 'axios'

const TOKEN_KEY = 'pawAdminToken'
const token = ref('')
const authed = ref(false)
const loading = ref(false)
const error = ref('')

const stats = ref(null)
const tasks = ref([])
const totalTasks = ref(0)
const detail = ref(null)
const deleting = ref(false)

// 小元件：水平長條圖
const BarList = (props) => {
  const data = props.data || []
  const max = Math.max(1, ...data.map(d => d.count))
  return h('div', { class: 'bars' }, data.map(d => h('div', { class: 'bar-row' }, [
    h('span', { class: 'bar-label' }, d.label),
    h('span', { class: 'bar-track' }, [h('span', { class: 'bar-fill', style: { width: (d.count / max * 100) + '%' } })]),
    h('span', { class: 'bar-count' }, d.count),
  ])))
}

const api = (path) => axios.get(`/api/admin${path}`, { headers: { 'X-Admin-Token': token.value } })

const login = async () => {
  if (!token.value) return
  loading.value = true; error.value = ''
  try {
    await api('/verify')
    localStorage.setItem(TOKEN_KEY, token.value)
    authed.value = true
    await loadAll()
  } catch (e) {
    error.value = e.response?.status === 401 ? 'Token 錯誤' : (e.response?.data?.error || '無法連線')
  } finally {
    loading.value = false
  }
}

const logout = () => {
  localStorage.removeItem(TOKEN_KEY)
  authed.value = false; token.value = ''; stats.value = null; tasks.value = []
}

const loadAll = async () => {
  const [s, t] = await Promise.all([api('/stats'), api('/tasks?limit=200')])
  stats.value = s.data
  tasks.value = t.data.tasks
  totalTasks.value = t.data.total
}

const openDetail = async (id) => {
  try { detail.value = (await api(`/tasks/${id}`)).data } catch (e) { alert('讀取失敗') }
}

const deleteTask = async (id) => {
  if (!confirm('確定刪除這個任務？最終影片也會一併刪掉，無法復原。')) return
  deleting.value = true
  try {
    await axios.delete(`/api/admin/tasks/${id}`, { headers: { 'X-Admin-Token': token.value } })
    detail.value = null
    await loadAll()
  } catch (e) { alert('刪除失敗') }
  finally { deleting.value = false }
}

const successRate = computed(() => stats.value && stats.value.total ? Math.round(stats.value.completed / stats.value.total * 100) : 0)
const sortedVideos = computed(() => detail.value ? [...detail.value.videos].sort((a, b) => a.video_index - b.video_index) : [])

const fmtDate = (s) => { if (!s) return '—'; const d = new Date(s); return `${d.getMonth()+1}/${d.getDate()} ${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}` }
const ms = (v) => v ? (v/1000).toFixed(1)+'s' : '—'
const modeLabel = (m) => ({ warm: '溫馨', cute: '活潑' }[m] || m || '—')
const statusLabel = (s) => ({ completed: '完成', failed: '失敗', analyzing: '分析中', generating_story: '寫劇本', generating_video: '合成中' }[s] || s)
const interLabel = (i) => ({ being_petted: '被摸', cuddling: '被抱', playing: '玩耍', fetching: '撿球', running_towards_owner: '奔向主人', none: '無互動' }[i] || i || '—')
const storageUrl = (p) => p ? '/storage/' + p.replace(/^.*storage\//, '') : ''

onMounted(() => {
  const saved = localStorage.getItem(TOKEN_KEY)
  if (saved) { token.value = saved; login() }
})
</script>

<style scoped>
.admin { max-width: 1100px; margin: 0 auto; padding: 1.5rem 1rem 4rem; color: #1e293b; }
.login-box { max-width: 360px; margin: 12vh auto; background: #fff; padding: 2rem; border-radius: 16px; box-shadow: 0 10px 40px rgba(0,0,0,.15); text-align: center; }
.login-box h1 { font-size: 1.4rem; margin-bottom: .5rem; }
.login-box input { width: 100%; padding: .8rem; margin: 1rem 0 .8rem; border: 2px solid #e2e8f0; border-radius: 10px; font-size: 1rem; }
.login-box button, .logout { background: #667eea; color: #fff; border: 0; padding: .7rem 1.4rem; border-radius: 10px; font-weight: 700; cursor: pointer; }
.hint { color: #64748b; font-size: .9rem; }
.err { color: #dc2626; font-size: .9rem; margin-top: .6rem; }
header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.2rem; }
header h1 { font-size: 1.3rem; }
.logout { background: #94a3b8; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px,1fr)); gap: .8rem; margin-bottom: 1.2rem; }
.card { background: #fff; border-radius: 14px; padding: 1rem; text-align: center; box-shadow: 0 2px 10px rgba(0,0,0,.06); }
.card .num { display: block; font-size: 1.8rem; font-weight: 800; }
.card .lbl { font-size: .8rem; color: #64748b; }
.card.ok .num { color: #16a34a; } .card.bad .num { color: #dc2626; }
.charts { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px,1fr)); gap: 1rem; margin-bottom: 1.5rem; }
.chart { background: #fff; border-radius: 14px; padding: 1rem 1.2rem; box-shadow: 0 2px 10px rgba(0,0,0,.06); }
.chart h3 { font-size: .95rem; margin-bottom: .6rem; }
:deep(.bar-row) { display: grid; grid-template-columns: 84px 1fr 32px; align-items: center; gap: .5rem; margin: .35rem 0; font-size: .82rem; }
:deep(.bar-label) { color: #475569; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
:deep(.bar-track) { background: #eef2ff; border-radius: 6px; height: 16px; overflow: hidden; }
:deep(.bar-fill) { display: block; height: 100%; background: linear-gradient(90deg,#667eea,#764ba2); }
:deep(.bar-count) { text-align: right; color: #475569; font-weight: 700; }
.tasks { background: #fff; border-radius: 14px; padding: 1rem 1.2rem; box-shadow: 0 2px 10px rgba(0,0,0,.06); }
.tasks h2 { font-size: 1.05rem; margin-bottom: .6rem; }
table { width: 100%; border-collapse: collapse; font-size: .85rem; }
th, td { text-align: left; padding: .55rem .4rem; border-bottom: 1px solid #f1f5f9; }
th { color: #64748b; font-weight: 600; }
.row { cursor: pointer; } .row:hover { background: #f8fafc; }
.empty { text-align: center; color: #94a3b8; padding: 1.5rem; }
.badge { padding: .15rem .5rem; border-radius: 6px; font-size: .75rem; font-weight: 700; background: #e2e8f0; }
.badge.completed { background: #dcfce7; color: #16a34a; }
.badge.failed { background: #fee2e2; color: #dc2626; }
.modal { position: fixed; inset: 0; background: rgba(0,0,0,.55); display: flex; align-items: flex-start; justify-content: center; padding: 4vh 1rem; z-index: 100; overflow: auto; }
.modal-box { background: #fff; border-radius: 16px; padding: 1.5rem; max-width: 760px; width: 100%; position: relative; }
.close { position: absolute; top: 1rem; right: 1rem; border: 0; background: #f1f5f9; width: 32px; height: 32px; border-radius: 50%; cursor: pointer; font-size: 1rem; }
.meta { color: #64748b; font-size: .85rem; margin: .4rem 0 1rem; }
.kv { display: grid; grid-template-columns: repeat(auto-fit,minmax(220px,1fr)); gap: .8rem; margin-bottom: 1rem; }
.kv > div { background: #f8fafc; border-radius: 10px; padding: .7rem .9rem; }
.kv b { font-size: .8rem; color: #64748b; } .kv p { margin-top: .3rem; font-size: .9rem; }
.watch { display: inline-block; background: #667eea; color: #fff; padding: .5rem 1rem; border-radius: 10px; text-decoration: none; font-weight: 700; margin-bottom: 1rem; }
.modal-box h3 { font-size: 1rem; margin: 1rem 0 .6rem; }
.vid { border: 1px solid #e2e8f0; border-radius: 12px; margin-bottom: .8rem; overflow: hidden; }
.vid-head { display: flex; gap: .6rem; align-items: center; background: #f8fafc; padding: .5rem .8rem; font-size: .82rem; }
.vidx { font-weight: 800; color: #667eea; }
.pos { background: #dbeafe; color: #1d4ed8; padding: .1rem .45rem; border-radius: 5px; font-size: .72rem; }
.pos.unused { background: #fee2e2; color: #dc2626; }
.fname { color: #475569; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.sz { color: #94a3b8; }
.vid-body { display: grid; grid-template-columns: 1fr 1fr; gap: 1px; background: #e2e8f0; }
.vid-body .col { background: #fff; padding: .7rem .8rem; }
.vid-body b { font-size: .8rem; color: #64748b; }
.caption { margin-top: .3rem; font-size: .88rem; }
.tags { margin-top: .4rem; display: flex; flex-wrap: wrap; gap: .3rem; }
.tag { background: #f1f5f9; border-radius: 5px; padding: .12rem .4rem; font-size: .72rem; color: #475569; }
.narration { margin-top: .3rem; font-size: .88rem; color: #0f172a; line-height: 1.5; }
.failed { margin-top: .3rem; font-size: .78rem; color: #b45309; background: #fef3c7; border-radius: 6px; padding: .25rem .5rem; }
.danger-zone { margin-top: 1.2rem; padding-top: 1rem; border-top: 1px solid #f1f5f9; text-align: center; }
.del-btn { background: #fee2e2; color: #dc2626; border: 1px solid #fecaca; padding: .6rem 1.2rem; border-radius: 10px; font-weight: 700; cursor: pointer; font-size: .9rem; }
.del-btn:hover { background: #dc2626; color: #fff; }
@media (max-width: 560px) { .vid-body { grid-template-columns: 1fr; } }
</style>
