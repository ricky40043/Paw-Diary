<template>
  <div class="love-story">
    <div class="container">
      <h1>🐾 狗狗回憶影片自動剪輯</h1>
      <p class="subtitle">珍藏與毛孩相處的每一個溫馨時刻</p>

      <!-- 步驟指示器 -->
      <div class="step-indicator">
        <div v-for="i in 4" :key="i" :class="['step-dot', { active: currentStep >= i, current: currentStep === i }]">
          <span class="step-num">{{ i }}</span>
        </div>
      </div>

      <!-- Step 1: 狗狗資訊與風格 -->
      <div v-if="currentStep === 1" class="step-card">
        <!-- 我做過的影片（存在本機，永久連結，滑掉也找得到） -->
        <div v-if="videoHistory.length" class="history-box">
          <h3>📼 我做過的影片</h3>
          <div v-for="item in videoHistory" :key="item.id" class="history-item">
            <div class="history-info">
              <span class="history-name">{{ item.name }}的回憶錄</span>
              <span class="history-date">{{ formatHistoryDate(item.ts) }}</span>
            </div>
            <a :href="item.url" target="_blank" rel="noopener" class="history-open">▶ 開啟</a>
            <a :href="item.url" :download="`${item.name}回憶錄.mp4`" class="history-open dl">⬇</a>
            <button type="button" class="history-del" @click="removeFromHistory(item.id)">✕</button>
          </div>
          <p class="history-hint">這些連結存在你的手機，滑掉或關掉都還能回來開／下載</p>
        </div>

        <h2>步驟 1：狗狗資訊</h2>
        <form @submit.prevent="createProject">
          <div class="form-group">
            <label>狗狗名字 * <span class="char-count">{{ dogName.length }}/10</span></label>
            <input 
              v-model="dogName" 
              type="text" 
              placeholder="例如：豆豆" 
              maxlength="10"
              required 
            />
          </div>
          
          <div class="form-group">
            <label>你的稱呼 (例如：媽媽、爸爸)</label>
            <input 
              v-model="ownerRelationship" 
              type="text" 
              placeholder="例如：媽媽" 
              maxlength="10"
            />
            <div class="relation-selector" style="margin-top: 0.5rem">
              <div 
                v-for="rel in relations" 
                :key="rel"
                :class="['relation-chip', { selected: ownerRelationship === rel }]"
                @click="ownerRelationship = rel"
              >
                {{ rel }}
              </div>
            </div>
          </div>

          <div class="form-group">
            <label>影片風格</label>
            <div class="mode-selector-mini">
              <div 
                v-for="mode in storyModes" 
                :key="mode.value"
                :class="['mode-chip', { selected: storyMode === mode.value }]"
                @click="storyMode = mode.value"
              >
                {{ mode.icon }} {{ mode.name }}
              </div>
            </div>
            <p class="mode-desc-text">{{ storyModes.find(m => m.value === storyMode)?.desc }}</p>
          </div>

          <div class="actions">
            <button 
              type="submit" 
              :disabled="!canProceedStep1"
              class="btn-primary"
            >
              下一步
            </button>
          </div>
        </form>
      </div>

      <!-- Step 2: 上傳影片 -->
      <div v-if="currentStep === 2" class="step-card">
        <h2>步驟 2：上傳影片</h2>
        <p class="hint">選擇 1～10 個與狗狗互動的溫馨片段，<strong>選好就會自動上傳</strong>，每段會被剪輯成約 12～20 秒（影片越多每段越短，總長控制在約 2 分鐘內）</p>

        <!-- 選擇區：用 label 觸發 input，iOS Safari 相容性最佳 -->
        <label
          for="videoInput"
          class="video-drop-zone"
          :class="{ disabled: videoCount >= 10 }"
          :style="videoCount >= 10 ? 'pointer-events: none' : 'cursor: pointer'"
        >
          <div class="icon">{{ videoCount >= 10 ? '✅' : '🎬' }}</div>
          <p>
            <span v-if="videoCount >= 10">已達上限 10 個</span>
            <span v-else-if="videoCount === 0">點擊選擇影片</span>
            <span v-else>已選 {{ videoCount }} 個，點擊繼續新增</span>
          </p>
          <p class="small">可一次多選，最多 10 個</p>
        </label>
        <input
          id="videoInput"
          ref="videoInput"
          type="file"
          accept="video/*"
          multiple
          @change="handleVideoSelect"
          style="display: none"
        />

        <!-- 已選影片清單（逐支顯示上傳狀態） -->
        <div v-if="videoCount > 0" class="video-list">
          <div v-for="(video, index) in videos" :key="video.key" class="video-list-item">
            <span class="video-num" :class="video.status">
              <template v-if="video.status === 'done'">✓</template>
              <template v-else-if="video.status === 'error'">!</template>
              <template v-else>{{ index + 1 }}</template>
            </span>
            <div class="video-info">
              <p class="name">{{ video.name }}</p>
              <p class="size">
                {{ formatFileSize(video.size) }}
                <span v-if="video.status === 'uploading'" class="st up">· 上傳中 {{ video.progress }}%</span>
                <span v-else-if="video.status === 'pending'" class="st wait">· 等待中…</span>
                <span v-else-if="video.status === 'done'" class="st ok">· ✅ 已上傳</span>
                <span v-else-if="video.status === 'error'" class="st err">· ❌ {{ video.error || '上傳失敗' }}</span>
              </p>
              <div v-if="video.status === 'uploading'" class="mini-progress">
                <div class="mini-progress-bar" :style="{ width: video.progress + '%' }"></div>
              </div>
            </div>
            <button v-if="video.status === 'error'" type="button" @click="retryUpload(index)" class="btn-retry">重試</button>
            <button type="button" @click="removeVideo(index)" class="btn-remove">✕</button>
          </div>
        </div>

        <p class="upload-hint">
          已上傳 {{ doneCount }} / {{ videoCount }} 個影片
          <span v-if="inFlightCount > 0">（一次傳一支，還有 {{ inFlightCount }} 支處理中…）</span>
        </p>

        <div class="actions">
          <button @click="currentStep = 1" class="btn-secondary">上一步</button>
          <button
            @click="currentStep = 3"
            :disabled="videoCount < 1"
            class="btn-primary"
          >
            下一步<span v-if="inFlightCount > 0">（{{ inFlightCount }} 支上傳中）</span>
          </button>
        </div>
      </div>

      <!-- Step 3: 上傳結尾圖片 -->
      <div v-if="currentStep === 3" class="step-card">
        <h2>步驟 3：上傳結尾圖片</h2>
        <p class="hint">選擇一張狗狗的照片作為影片結尾，<strong>選好就會自動上傳</strong></p>

        <div class="image-upload">
          <div v-if="!endingImage && !endingUploaded" class="upload-placeholder" @click="$refs.imageInput.click()">
            <div class="icon">🖼️</div>
            <p>選擇圖片</p>
          </div>
          <div v-else class="image-preview">
            <img v-if="imagePreview" :src="imagePreview" alt="結尾圖片" />
            <p v-else class="hint" style="text-align:center">✅ 已上傳結尾圖片</p>
            <template v-if="uploadingImage">
              <p class="upload-hint st up">上傳中… {{ imageProgress }}%</p>
              <div class="mini-progress"><div class="mini-progress-bar" :style="{ width: imageProgress + '%' }"></div></div>
            </template>
            <p v-else-if="endingUploaded" class="upload-hint" style="color:#4caf50">✅ 上傳完成</p>
            <button type="button" @click="$refs.imageInput.click()" class="btn-remove" :disabled="uploadingImage">更換圖片</button>
          </div>
        </div>
        <input
          ref="imageInput"
          type="file"
          accept="image/*"
          @change="handleImageSelect"
          style="display: none"
        />

        <div class="actions">
          <button @click="currentStep = 2" class="btn-secondary">上一步</button>
          <button
            @click="currentStep = 4"
            :disabled="!imageSelected"
            class="btn-primary"
          >
            下一步<span v-if="uploadingImage">（圖片上傳中）</span>
          </button>
        </div>
      </div>

      <!-- Step 4: 主人留言 -->
      <div v-if="currentStep === 4" class="step-card">
        <h2>步驟 4：要對狗狗說的話</h2>
        <p class="hint">寫下你想對狗狗說的話，這將會成為影片中感人的一幕</p>
        
        <div class="form-group">
          <label>給寶貝的一句話 * <span class="char-count">{{ ownerMessage.length }}/100</span></label>
          <textarea 
            v-model="ownerMessage" 
            rows="4" 
            placeholder="例如：謝謝你來到我的生命中，你是我最好的朋友..." 
            class="message-input"
            maxlength="100"
            required
          ></textarea>
          <p v-if="ownerMessage.length === 0" class="error-hint">請輸入給狗狗的話</p>
          <p v-else-if="ownerMessage.length < 10" class="error-hint">至少輸入 10 個字</p>
        </div>

        <!-- 按下開始製作後，若還在上傳就顯示狀態，完成後自動開始（不會卡住操作） -->
        <div v-if="finalizing" class="upload-wait">
          <div class="spinner small"></div>
          <p v-if="inFlightCount > 0 || uploadingImage">
            📤 影片上傳中，完成後會自動開始製作…<br>
            <span class="hint">影片 {{ doneCount }}/{{ videoCount }} 已上傳{{ uploadingImage ? '、結尾圖上傳中' : '' }}</span>
          </p>
          <p v-else>✅ 上傳完成，正在開始製作…</p>
        </div>

        <div class="actions">
          <button @click="currentStep = 3" class="btn-secondary" :disabled="finalizing">上一步</button>
          <button
            @click="submitOwnerMessage"
            :disabled="!canSubmitMessage || finalizing"
            class="btn-primary"
          >
            {{ finalizing ? (inFlightCount > 0 || uploadingImage ? '上傳中…' : '開始製作…') : '開始製作影片' }}
          </button>
        </div>
      </div>

      <!-- Step 5: 處理中 -->
      <div v-if="currentStep === 5" class="step-card processing">
        <div class="spinner"></div>
        <h2>✨ 正在製作影片... ({{ progress }}%)</h2>
        <p class="status-main">{{ statusMessage }}</p>
        <div class="progress">
          <div class="progress-bar" :style="{width: progress + '%'}"></div>
        </div>
      </div>

      <!-- Step 6: 完成 -->
      <div v-if="currentStep === 6" class="step-card completed">
        <div v-if="result" class="result">
          <div class="video-player">
            <video controls :src="result.final_video_url">
              您的瀏覽器不支援影片播放
            </video>
          </div>

          <div class="actions">
            <a :href="result.final_video_url" :download="`${dogName}回憶錄.mp4`" class="btn-primary">
              ⬇️ 下載影片 (`{{ dogName }}回憶錄.mp4`)
            </a>
            <button @click="reset" class="btn-secondary">製作新的影片</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import axios from 'axios'

// 步驟狀態
const currentStep = ref(1)

// Step 1: 資訊與風格
const storyMode = ref('warm')
const storyModes = [
  { value: 'warm', name: '溫馨感人', icon: '💝', desc: '溫柔感性、充滿愛的表達' },
  { value: 'cute', name: '可愛活潑', icon: '🐾', desc: '活潑撒嬌、充滿元氣' }
]

const dogName = ref('')
const dogBreed = ref('') // 保留變數但UI隱藏
const ownerRelationship = ref('媽媽')
const relations = ['爸爸', '媽媽', '哥哥', '姊姊', '弟弟', '妹妹', '主人']

// Step 2: 影片（每支獨立狀態，選完即自動上傳）
// 每筆：{ key, id, name, size, status: 'uploading'|'done'|'error', progress, error, file }
const videos = ref([])
const videoInput = ref(null)
const addingVideo = ref(false)

// Step 3: 圖片
const endingImage = ref(null)
const imagePreview = ref('')
const imageInput = ref(null)
const uploadingImage = ref(false)
const imageProgress = ref(0)
const endingUploaded = ref(false)
const imageSelected = ref(false) // 已選圖片（即使還在上傳也可進下一步）

// Step 4: 留言
const ownerMessage = ref('')
const finalizing = ref(false) // 按下開始製作後，等待上傳完成→開始製作的過渡狀態

// 專案
const projectId = ref('')
const statusMessage = ref('')
const progress = ref(0)
const result = ref(null)
let pollTimer = null

// ---------------------------------------------------------------------------
// 「我的影片」歷史：完成的影片永久連結存在本機。影片檔在伺服器的 docker volume，
// 重新部署也不會消失，所以這個直接連結滑掉/關瀏覽器/重新部署後都還能開。
// ---------------------------------------------------------------------------
const HISTORY_KEY = 'pawDiaryHistory'
const videoHistory = ref([])

const loadHistory = () => {
  try { videoHistory.value = JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]') } catch (e) { videoHistory.value = [] }
}
const addToHistory = (id, name, relativeUrl) => {
  if (!id || !relativeUrl) return
  const absUrl = relativeUrl.startsWith('http') ? relativeUrl : (window.location.origin + relativeUrl)
  const list = videoHistory.value.filter(v => v.id !== id)
  list.unshift({ id, name: name || '毛孩', url: absUrl, ts: Date.now() })
  videoHistory.value = list.slice(0, 30)
  try { localStorage.setItem(HISTORY_KEY, JSON.stringify(videoHistory.value)) } catch (e) {}
}
const removeFromHistory = (id) => {
  videoHistory.value = videoHistory.value.filter(v => v.id !== id)
  try { localStorage.setItem(HISTORY_KEY, JSON.stringify(videoHistory.value)) } catch (e) {}
}
const formatHistoryDate = (ts) => {
  const d = new Date(ts)
  return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

// ---------------------------------------------------------------------------
// localStorage 狀態保存：手機不小心滑掉 / 重新整理也不會全部消失
// （已上傳的影片存在伺服器，用 id 追蹤；File 本體無法保存，但不需要重傳）
// ---------------------------------------------------------------------------
const STORE_KEY = 'pawDiaryState'

const persist = () => {
  try {
    const snapshot = {
      currentStep: currentStep.value,
      projectId: projectId.value,
      dogName: dogName.value,
      dogBreed: dogBreed.value,
      ownerRelationship: ownerRelationship.value,
      storyMode: storyMode.value,
      ownerMessage: ownerMessage.value,
      endingUploaded: endingUploaded.value,
      // 只保存已上傳成功的影片（去掉 File 本體）
      videos: videos.value
        .filter(v => v.status === 'done' && v.id)
        .map(v => ({ key: v.key, id: v.id, name: v.name, size: v.size, status: 'done', progress: 100 }))
    }
    localStorage.setItem(STORE_KEY, JSON.stringify(snapshot))
  } catch (e) { /* 隱私模式可能無法寫入，忽略 */ }
}

const clearPersist = () => {
  try { localStorage.removeItem(STORE_KEY) } catch (e) {}
}

const restore = async () => {
  let snap
  try { snap = JSON.parse(localStorage.getItem(STORE_KEY) || 'null') } catch (e) { snap = null }
  if (!snap || !snap.projectId) return

  // 先確認伺服器上的專案還在（伺服器重啟會清空），否則放棄還原
  let server
  try {
    const res = await axios.get(`/api/v2/story/projects/${snap.projectId}`)
    server = res.data
  } catch (e) {
    clearPersist()
    return
  }

  projectId.value = snap.projectId
  dogName.value = snap.dogName || ''
  dogBreed.value = snap.dogBreed || ''
  ownerRelationship.value = snap.ownerRelationship || '媽媽'
  storyMode.value = snap.storyMode || 'warm'
  ownerMessage.value = snap.ownerMessage || ''
  endingUploaded.value = !!snap.endingUploaded
  imageSelected.value = !!snap.endingUploaded

  // 以伺服器實際擁有的影片為準（避免顯示已被刪除的）
  const serverIds = new Set((server.videos || []).map(v => v.id))
  videos.value = (snap.videos || [])
    .filter(v => serverIds.has(v.id))
    .map(v => ({ ...v, file: null, error: '' }))

  const status = server.status
  if (status === 'completed') {
    result.value = server
    currentStep.value = 6
  } else if (status === 'analyzing' || status === 'generating_story' || status === 'generating_video' || status === 'processing') {
    currentStep.value = 5
    pollProgress()
  } else {
    currentStep.value = Math.min(snap.currentStep || 1, 4)
  }
}

onMounted(() => { loadHistory(); restore() })

// 任一關鍵狀態變動就保存（videos 不放進 watch，改在上傳完成/移除時明確 persist，
// 避免上傳進度每跳一格就寫一次 localStorage 造成手機卡頓）
watch([currentStep, projectId, dogName, ownerRelationship, storyMode, ownerMessage, endingUploaded], persist)

// 計算屬性
const canProceedStep1 = computed(() =>
  dogName.value.trim().length > 0 && ownerRelationship.value !== '' && !!storyMode.value)

const videoCount = computed(() => videos.value.length)
const doneCount = computed(() => videos.value.filter(v => v.status === 'done').length)
const uploadingCount = computed(() => videos.value.filter(v => v.status === 'uploading').length)
const pendingCount = computed(() => videos.value.filter(v => v.status === 'pending').length)
const inFlightCount = computed(() => uploadingCount.value + pendingCount.value)
const canProceedStep2 = computed(() => doneCount.value >= 1 && inFlightCount.value === 0)
const canSubmitMessage = computed(() => ownerMessage.value.trim().length >= 10)

const createProject = async () => {
  if (!canProceedStep1.value) return
  try {
    // 已有專案就重用，避免返回上一步又建立重複專案
    if (!projectId.value) {
      const response = await axios.post('/api/v2/story/projects', {
        name: `${dogName.value}的告白`,
        dog_name: dogName.value,
        dog_breed: dogBreed.value,
        owner_relationship: ownerRelationship.value,
        story_mode: storyMode.value
      })
      projectId.value = response.data.project_id
    }
    currentStep.value = 2
  } catch (error) {
    alert('建立專案失敗：' + (error.response?.data?.error || error.message))
  }
}

const formatFileSize = (bytes) => {
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

const CHUNK_SIZE = 5 * 1024 * 1024 // 5MB：手機與反向代理友善，進度更新更頻繁

// 分塊上傳：逐塊「序列」送出，並用 onUploadProgress 提供位元組級即時進度
const uploadFileChunked = async (pid, entry) => {
  const file = entry.file
  const totalChunks = Math.ceil(file.size / CHUNK_SIZE)
  const fileId = Date.now().toString(36) + Math.random().toString(36).substr(2)
  let assembled = null

  for (let j = 0; j < totalChunks; j++) {
    const start = j * CHUNK_SIZE
    const end = Math.min(start + CHUNK_SIZE, file.size)
    const chunk = file.slice(start, end)
    const formData = new FormData()
    formData.append('chunk', chunk)
    formData.append('file_id', fileId)
    formData.append('chunk_index', j)
    formData.append('total_chunks', totalChunks)
    formData.append('filename', file.name)
    const res = await axios.post(`/api/v2/story/projects/${pid}/video-chunk`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000, // 單一 5MB 分塊逾時保護，避免無限卡 0%
      onUploadProgress: (e) => {
        // 已送出的分塊位元組 + 目前分塊已上傳位元組 → 平滑整體百分比
        const uploaded = start + (e.loaded || 0)
        entry.progress = Math.min(99, Math.round((uploaded / file.size) * 100))
      }
    })
    if (res.data && res.data.assembled) assembled = res.data.video
  }
  return assembled
}

// 上傳單一影片（自動判斷是否分塊），逐支回報狀態
const uploadOne = async (entry) => {
  if (!projectId.value || !entry.file) return
  entry.status = 'uploading'
  entry.progress = 0
  entry.error = ''
  try {
    let video
    // 大於 8MB 走分塊（每塊 5MB），避免手機/代理對單一大請求的限制
    if (entry.file.size > 8 * 1024 * 1024) {
      video = await uploadFileChunked(projectId.value, entry)
    } else {
      const formData = new FormData()
      formData.append('videos', entry.file)
      const res = await axios.post(`/api/v2/story/projects/${projectId.value}/videos`, formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 120000,
        onUploadProgress: (e) => {
          if (e.total) entry.progress = Math.min(99, Math.round((e.loaded / e.total) * 100))
        }
      })
      video = res.data?.videos?.[0]
    }
    if (video && video.id) entry.id = video.id
    entry.status = 'done'
    entry.progress = 100
    entry.file = null // 釋放記憶體，且不會再被重傳
    persist()
  } catch (error) {
    entry.status = 'error'
    entry.error = error.response?.data?.error || error.message || '上傳失敗'
  }
}

// 序列式上傳佇列：一次只傳一支，其餘排隊「等待中」
let uploadRunning = false
const processUploadQueue = async () => {
  if (uploadRunning) return
  uploadRunning = true
  try {
    while (true) {
      const next = videos.value.find(v => v.status === 'pending' && v.file)
      if (!next) break
      await uploadOne(next)
    }
  } finally {
    uploadRunning = false
  }
}

const handleVideoSelect = (event) => {
  addingVideo.value = true
  const newFiles = Array.from(event.target.files || [])
  const seen = new Set(videos.value.map(v => v.name + v.size))
  for (const f of newFiles) {
    const key = f.name + f.size
    if (!seen.has(key) && videos.value.length < 5) {
      seen.add(key)
      // 先排入佇列（等待中），由 processUploadQueue 一支一支上傳
      videos.value.push({ key: key + Math.random().toString(36).slice(2), id: '', name: f.name, size: f.size, status: 'pending', progress: 0, error: '', file: f })
    }
  }
  event.target.value = ''
  addingVideo.value = false
  processUploadQueue()
}

const retryUpload = (index) => {
  const entry = videos.value[index]
  if (entry && entry.file) {
    entry.status = 'pending'
    entry.error = ''
    processUploadQueue()
  }
}

const removeVideo = async (index) => {
  const entry = videos.value[index]
  videos.value.splice(index, 1)
  persist()
  // 已上傳到伺服器的，連同伺服器檔案一起刪除
  if (entry && entry.id && projectId.value) {
    try {
      await axios.delete(`/api/v2/story/projects/${projectId.value}/videos/${entry.id}`)
    } catch (e) { /* 忽略刪除失敗 */ }
  }
}

// 上傳前在手機端把圖片縮小（最長邊 maxSize、JPEG），原圖可能 5–10MB，
// 縮完只剩幾百 KB，慢速行動網路也能秒傳（最終影片 720p，畫質無差）。
// iOS Safari 可解 HEIC；若解碼失敗就退回原檔。
const downscaleImage = (file, maxSize = 1600, quality = 0.85) => new Promise((resolve) => {
  if (!file || !file.type || !file.type.startsWith('image/')) return resolve(file)
  const url = URL.createObjectURL(file)
  const img = new Image()
  img.onload = () => {
    URL.revokeObjectURL(url)
    let { width, height } = img
    if (!width || !height) return resolve(file)
    const longest = Math.max(width, height)
    if (longest > maxSize) {
      const r = maxSize / longest
      width = Math.round(width * r); height = Math.round(height * r)
    }
    try {
      const canvas = document.createElement('canvas')
      canvas.width = width; canvas.height = height
      canvas.getContext('2d').drawImage(img, 0, 0, width, height)
      canvas.toBlob((blob) => {
        if (blob && blob.size > 0) {
          const name = (file.name || 'ending').replace(/\.[^.]+$/, '') + '.jpg'
          resolve(new File([blob], name, { type: 'image/jpeg' }))
        } else resolve(file)
      }, 'image/jpeg', quality)
    } catch (e) { resolve(file) }
  }
  img.onerror = () => { URL.revokeObjectURL(url); resolve(file) }
  img.src = url
})

const handleImageSelect = async (event) => {
  const file = event.target.files[0]
  event.target.value = ''
  if (!file) return
  imageSelected.value = true
  imagePreview.value = URL.createObjectURL(file)
  endingUploaded.value = false
  uploadingImage.value = true
  imageProgress.value = 0
  try {
    // 先縮圖再上傳
    const smaller = await downscaleImage(file)
    endingImage.value = smaller
    const formData = new FormData()
    formData.append('image', smaller)
    await axios.post(`/api/v2/story/projects/${projectId.value}/ending-image`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000,
      onUploadProgress: (e) => {
        if (e.total) imageProgress.value = Math.min(99, Math.round((e.loaded / e.total) * 100))
      }
    })
    imageProgress.value = 100
    endingUploaded.value = true
    persist()
  } catch (error) {
    alert('上傳圖片失敗：' + (error.response?.data?.error || error.message || '請重試'))
  } finally {
    uploadingImage.value = false
  }
}

const removeImage = () => {
  endingImage.value = null
  imagePreview.value = ''
  endingUploaded.value = false
  imageSelected.value = false
}

// 等待所有上傳（影片佇列 + 結尾圖）完成；期間畫面會顯示「上傳中」。
const waitForUploads = () => new Promise((resolve) => {
  const check = () => {
    if (inFlightCount.value === 0 && !uploadingImage.value) resolve()
    else setTimeout(check, 400)
  }
  check()
})

const submitOwnerMessage = async () => {
  if (!canSubmitMessage.value || finalizing.value) return
  finalizing.value = true
  try {
    // 1) 先存留言（不需等上傳）
    await axios.post(`/api/v2/story/projects/${projectId.value}/owner-message`, {
      message: ownerMessage.value
    })
    // 2) 等所有上傳完成（若早就傳完，這裡幾乎是瞬間）
    await waitForUploads()
    // 3) 驗證上傳結果，有問題就提示返回對應步驟重試
    if (doneCount.value < 1) {
      alert('影片尚未成功上傳，請返回步驟 2 確認或重試')
      return
    }
    if (!endingUploaded.value) {
      alert('結尾圖片尚未成功上傳，請返回步驟 3 重試')
      return
    }
    // 4) 開始製作
    await axios.post(`/api/v2/story/projects/${projectId.value}/generate`)
    currentStep.value = 5
    pollProgress()
  } catch (error) {
    alert('提交失敗：' + (error.response?.data?.error || error.message))
  } finally {
    finalizing.value = false
  }
}

// 進度查詢：單調遞增、不倒退，並依後端實際 progress 平滑顯示
const STATUS_TEXT = {
  analyzing: '正在分析影片內容…',
  generating_story: '正在創作對白…',
  generating_video: '正在合成最終影片…',
  processing: '處理中…'
}
const STATUS_FLOOR = { analyzing: 8, generating_story: 50, generating_video: 70, processing: 5 }

const pollProgress = () => {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = setInterval(async () => {
    try {
      const response = await axios.get(`/api/v2/story/projects/${projectId.value}`)
      const status = response.data.status
      const serverProgress = Number(response.data.progress) || 0

      if (status === 'completed') {
        clearInterval(pollTimer); pollTimer = null
        progress.value = 100
        result.value = response.data
        // 存進「我的影片」永久清單（直接連結，滑掉/重新部署都還能開）
        addToHistory(projectId.value, dogName.value, response.data.final_video_url)
        clearPersist()
        setTimeout(() => { currentStep.value = 6 }, 500)
        return
      }
      if (status === 'failed') {
        clearInterval(pollTimer); pollTimer = null
        alert('處理失敗：' + (response.data.error || '未知錯誤'))
        currentStep.value = 4
        return
      }

      statusMessage.value = STATUS_TEXT[status] || '處理中…'
      // 以「後端進度」與「該階段下限」取較大值，且永不低於目前顯示值（不倒退）
      const target = Math.max(serverProgress, STATUS_FLOOR[status] || 0)
      if (target > progress.value) progress.value = Math.min(target, 99)
    } catch (error) {
      console.error('查詢進度失敗:', error)
    }
  }, 2000)
}

const reset = () => {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  clearPersist()
  currentStep.value = 1
  storyMode.value = 'warm'
  dogName.value = ''
  dogBreed.value = ''
  ownerRelationship.value = '媽媽'
  projectId.value = ''
  ownerMessage.value = ''
  videos.value = []
  endingImage.value = null
  imagePreview.value = ''
  endingUploaded.value = false
  result.value = null
  progress.value = 0
}
</script>

<style scoped>
.love-story {
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 2rem;
}

@media (max-width: 768px) {
  .love-story {
    padding: 0;
  }
}

.container {
  max-width: 1000px;
  margin: 0 auto;
}

h1 {
  color: white;
  text-align: center;
  font-size: 2.5rem;
  margin-bottom: 0.5rem;
  text-shadow: 2px 2px 4px rgba(0,0,0,0.2);
}

.subtitle {
  color: white;
  text-align: center;
  font-size: 1.2rem;
  margin-bottom: 2rem;
  opacity: 0.9;
}

/* 步驟指示器 */
.step-indicator {
  display: flex;
  justify-content: center;
  gap: 1rem;
  margin-bottom: 2rem;
}

.step-dot {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: rgba(255,255,255,0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s;
}

.step-dot.active {
  background: white;
}

.step-dot.current {
  transform: scale(1.2);
  box-shadow: 0 0 15px rgba(255,255,255,0.5);
}

.step-num {
  font-weight: 600;
  color: #667eea;
}

.step-dot:not(.active) .step-num {
  color: white;
}

/* 卡片 */
.step-card {
  background: white;
  border-radius: 20px;
  padding: 2.5rem;
  box-shadow: 0 10px 40px rgba(0,0,0,0.2);
}

@media (max-width: 768px) {
  .step-card {
    padding: 1.5rem 1rem;
    border-radius: 0; /* Full width look */
    min-height: 100vh; /* Make card take full height */
    box-shadow: none;
  }
  
  /* Add top padding for content */
  .container {
    padding-top: 1rem;
  }
}

h2 {
  color: #667eea;
  margin-bottom: 1.5rem;
}

.hint {
  color: #666;
  margin-bottom: 1.5rem;
}

/* 風格選擇 */
.mode-selector {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1.5rem;
  margin-bottom: 2rem;
}

.mode-card {
  border: 3px solid #e0e0e0;
  border-radius: 15px;
  padding: 1.5rem;
  text-align: center;
  cursor: pointer;
  transition: all 0.3s;
}

.mode-card:hover {
  border-color: #667eea;
  transform: translateY(-5px);
}

.mode-card.selected {
  border-color: #667eea;
  background: linear-gradient(135deg, #667eea10 0%, #764ba210 100%);
}

.mode-icon {
  font-size: 3rem;
  margin-bottom: 0.5rem;
}

.mode-name {
  font-size: 1.2rem;
  font-weight: 600;
  color: #333;
  margin-bottom: 0.5rem;
}

.mode-desc {
  font-size: 0.9rem;
  color: #666;
}

/* 表單 */
.form-group {
  margin-bottom: 1.5rem;
}

.form-group label {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
  font-weight: 600;
  color: #333;
}

.char-count {
  font-weight: normal;
  font-size: 0.85rem;
  color: #999;
}

.form-group input {
  width: 100%;
  padding: 0.8rem;
  border: 2px solid #e0e0e0;
  border-radius: 8px;
  font-size: 1rem;
}

.form-group input:focus,
.form-group textarea:focus {
  outline: none;
  border-color: #667eea;
}

.error-hint {
  color: #f44336;
  font-size: 0.85rem;
  margin-top: 0.3rem;
}

/* 關係選擇 */
.relation-selector {
  display: flex;
  flex-wrap: wrap;
  gap: 0.8rem;
}

.relation-chip {
  padding: 0.6rem 1.2rem;
  border: 2px solid #e0e0e0;
  border-radius: 25px;
  cursor: pointer;
  transition: all 0.3s;
  font-weight: 500;
}

.relation-chip:hover {
  border-color: #667eea;
}

.relation-chip.selected {
  background: #667eea;
  border-color: #667eea;
  color: white;
}

/* 影片上傳 */
.video-drop-zone {
  /* label 預設是 inline，有邊框時跨行會把虛線框拆成碎片；改 flex 讓它變成完整方塊 */
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border: 3px dashed #667eea;
  border-radius: 15px;
  padding: 2.5rem 1rem;
  text-align: center;
  cursor: pointer;
  transition: all 0.3s;
  margin-bottom: 1.2rem;
}

.video-drop-zone:hover:not(.disabled) {
  background: #f8f9ff;
  border-color: #764ba2;
}

.video-drop-zone.disabled {
  border-color: #ccc;
  cursor: default;
  opacity: 0.6;
}

.mini-spinner {
  width: 32px;
  height: 32px;
  border: 4px solid #e0e0e0;
  border-top: 4px solid #667eea;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin: 0 auto 0.5rem auto;
}

.icon {
  font-size: 2.5rem;
  margin-bottom: 0.5rem;
}

.small {
  color: #999;
  font-size: 0.8rem;
}

/* 已選影片清單 */
.video-list {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  margin-bottom: 1rem;
}

.video-list-item {
  display: flex;
  align-items: center;
  gap: 0.8rem;
  background: #f8f9ff;
  border-radius: 10px;
  padding: 0.7rem 1rem;
}

.video-num {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  background: #667eea;
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.8rem;
  font-weight: 600;
}

.video-info {
  flex: 1;
  min-width: 0;
}

.video-info .name {
  font-weight: 600;
  color: #333;
  font-size: 0.9rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin: 0;
}

.video-info .size {
  color: #888;
  font-size: 0.75rem;
  margin: 0;
}

.video-num.done { background: #4caf50; }
.video-num.error { background: #f44336; }
.video-num.uploading { background: #ff9800; }
.video-num.pending { background: #b0b6c8; }

.st { font-weight: 600; }
.st.up { color: #ff9800; }
.st.wait { color: #999; }
.st.ok { color: #4caf50; }
.st.err { color: #f44336; }

.mini-progress {
  height: 5px;
  background: #e6e9f5;
  border-radius: 5px;
  overflow: hidden;
  margin-top: 0.35rem;
}

.mini-progress-bar {
  height: 100%;
  background: linear-gradient(90deg, #667eea, #764ba2);
  transition: width 0.25s;
}

.btn-retry {
  background: #ff9800;
  color: white;
  font-size: 0.8rem;
  padding: 0.4rem 0.8rem;
  border: none;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  flex-shrink: 0;
}

.btn-retry:hover { background: #f57c00; }

/* 我做過的影片 */
.history-box {
  background: #f4f6ff;
  border: 1px solid #e0e6ff;
  border-radius: 12px;
  padding: 1rem;
  margin-bottom: 1.5rem;
}
.history-box h3 {
  color: #667eea;
  font-size: 1rem;
  margin: 0 0 0.6rem;
}
.history-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0;
  border-top: 1px solid #e6eaff;
}
.history-item:first-of-type { border-top: none; }
.history-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.history-name {
  font-weight: 600;
  color: #333;
  font-size: 0.9rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.history-date { color: #999; font-size: 0.75rem; }
.history-open {
  flex-shrink: 0;
  background: #667eea;
  color: #fff;
  text-decoration: none;
  padding: 0.35rem 0.7rem;
  border-radius: 8px;
  font-size: 0.85rem;
  font-weight: 600;
}
.history-open.dl { background: #5a6acf; padding: 0.35rem 0.55rem; }
.history-del {
  flex-shrink: 0;
  background: transparent;
  border: none;
  color: #bbb;
  font-size: 0.9rem;
  cursor: pointer;
  padding: 0.2rem 0.3rem;
}
.history-hint { color: #888; font-size: 0.75rem; margin: 0.5rem 0 0; }

.upload-hint {
  text-align: center;
  color: #666;
  margin-bottom: 1.5rem;
}

.upload-hint.warn {
  color: #f44336;
  font-weight: 600;
}

/* 圖片上傳 */
.image-upload {
  max-width: 400px;
  margin: 0 auto 2rem auto;
}

.image-preview img {
  width: 100%;
  border-radius: 15px;
  margin-bottom: 1rem;
}

.message-input {
  width: 100%;
  padding: 0.8rem;
  border: 2px solid #e0e0e0;
  border-radius: 8px;
  font-size: 1rem;
  resize: vertical;
  font-family: inherit;
}

/* 按鈕 */
.actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  margin-top: 1.5rem;
}

.btn-primary, .btn-secondary, .btn-remove {
  padding: 0.8rem 2rem;
  border: none;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s;
  text-decoration: none;
  display: inline-block;
}

.btn-primary {
  background: #667eea;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #5568d3;
  transform: translateY(-2px);
}

.btn-primary:disabled {
  background: #ccc;
  cursor: not-allowed;
}

.btn-secondary {
  background: #f0f0f0;
  color: #333;
}

.btn-secondary:hover {
  background: #e0e0e0;
}

.btn-remove {
  background: #f44336;
  color: white;
  font-size: 0.8rem;
  padding: 0.4rem 0.8rem;
  margin-top: 0.5rem;
}

.btn-remove:hover {
  background: #da190b;
}

/* 迷你風格選擇器 */
.mode-selector-mini {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
}

.mode-chip {
  padding: 0.6rem 1.2rem;
  border: 2px solid #e0e0e0;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.3s;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.mode-chip:hover {
  border-color: #667eea;
}

.mode-chip.selected {
  background: #667eea;
  border-color: #667eea;
  color: white;
}

.mode-desc-text {
  font-size: 0.85rem;
  color: #666;
  margin-top: 0.5rem;
  margin-left: 0.2rem;
}


/* 處理中 */
.processing {
  text-align: center;
  padding: 3rem;
}

.spinner {
  width: 60px;
  height: 60px;
  border: 6px solid #f3f3f3;
  border-top: 6px solid #667eea;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto 2rem auto;
}

.spinner.small {
  width: 26px;
  height: 26px;
  border-width: 3px;
  border-top-width: 3px;
  margin: 0 auto 0.5rem auto;
}

.upload-wait {
  margin: 0.5rem 0 1rem;
  padding: 1rem;
  border-radius: 12px;
  background: #f3f6ff;
  border: 1px solid #d9e2ff;
  text-align: center;
  color: #44506b;
  font-size: 0.95rem;
  line-height: 1.6;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.progress {
  width: 100%;
  height: 10px;
  background: #f0f0f0;
  border-radius: 10px;
  overflow: hidden;
  margin-top: 1rem;
}

.progress-bar {
  height: 100%;
  background: linear-gradient(90deg, #667eea, #764ba2);
  transition: width 0.5s;
}

/* 完成頁 */
.completed h2 {
  text-align: center;
  font-size: 2rem;
}

.result h3 {
  color: #667eea;
  text-align: center;
  margin-bottom: 1.5rem;
}

.chapters {
  margin: 2rem 0;
}

.chapter {
  display: flex;
  gap: 1rem;
  margin-bottom: 1rem;
  padding: 1rem;
  background: #f8f9ff;
  border-radius: 10px;
}

.chapter .index {
  flex-shrink: 0;
  width: 30px;
  height: 30px;
  background: #667eea;
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
}

.chapter p {
  flex: 1;
  color: #333;
  line-height: 1.6;
  margin: 0;
}

.final-message {
  text-align: center;
  padding: 1.5rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 15px;
  margin: 2rem 0;
}

.final-message p {
  color: white;
  font-size: 1.2rem;
  font-weight: 600;
  margin: 0;
}

.video-player {
  margin: 2rem 0;
}

.video-player video {
  width: 100%;
  max-width: 800px;
  display: block;
  margin: 0 auto;
  border-radius: 15px;
  box-shadow: 0 10px 30px rgba(0,0,0,0.2);
}
</style>
