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
        <p class="hint">選擇 1～5 個與狗狗互動的溫馨片段，每個影片會被剪輯成約 15 秒</p>

        <!-- 選擇區：每次點擊加入一支，手機相容 -->
        <div
          class="video-drop-zone"
          :class="{ disabled: videoCount >= 5 }"
          @click="videoCount < 5 && $refs.videoInput.click()"
        >
          <div class="icon">{{ videoCount >= 5 ? '✅' : '🎬' }}</div>
          <p>{{ videoCount >= 5 ? '已達上限 5 個' : `點擊新增影片 (${videoCount}/5)` }}</p>
          <p class="small">支援 MP4 / MOV / AVI，可多次點擊累加</p>
        </div>
        <input
          ref="videoInput"
          type="file"
          accept="video/*"
          @change="handleVideoSelect"
          style="display: none"
        />

        <!-- 已選影片清單 -->
        <div v-if="videoCount > 0" class="video-list">
          <div v-for="(video, index) in videos" :key="index" class="video-list-item">
            <span class="video-num">{{ index + 1 }}</span>
            <div class="video-info">
              <p class="name">{{ video.name }}</p>
              <p class="size">{{ formatFileSize(video.size) }}</p>
            </div>
            <button type="button" @click="removeVideo(index)" class="btn-remove">✕</button>
          </div>
        </div>

        <p class="upload-hint" :class="{ warn: videoCount > 5 }">
          已選擇 {{ videoCount }} / 5 個影片
          <span v-if="videoCount > 5">（超過上限，請移除多餘的）</span>
        </p>

        <div class="actions">
          <button @click="currentStep = 1" class="btn-secondary">上一步</button>
          <button
            @click="uploadVideos"
            :disabled="videoCount < 1 || videoCount > 5 || uploading"
            class="btn-primary"
          >
            {{ uploading ? `上傳中 ${uploadProgress}%` : '上傳影片' }}
          </button>
        </div>
      </div>

      <!-- Step 3: 上傳結尾圖片 -->
      <div v-if="currentStep === 3" class="step-card">
        <h2>步驟 3：上傳結尾圖片</h2>
        <p class="hint">選擇一張狗狗的照片作為影片結尾</p>
        
        <div class="image-upload">
          <div v-if="!endingImage" class="upload-placeholder" @click="$refs.imageInput.click()">
            <div class="icon">🖼️</div>
            <p>選擇圖片</p>
          </div>
          <div v-else class="image-preview">
            <img :src="imagePreview" alt="結尾圖片" />
            <button type="button" @click="removeImage" class="btn-remove">更換圖片</button>
          </div>
        </div>
        <input 
          ref="imageInput" 
          type="file" 
          accept="image/jpeg,image/jpg,image/png" 
          @change="handleImageSelect" 
          style="display: none"
        />
        
        <div class="actions">
          <button @click="currentStep = 2" class="btn-secondary">上一步</button>
          <button 
            @click="uploadImage" 
            :disabled="!endingImage || uploadingImage"
            class="btn-primary"
          >
            {{ uploadingImage ? '上傳中...' : '下一步' }}
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
        
        <div class="actions">
          <button @click="currentStep = 3" class="btn-secondary">上一步</button>
          <button 
            @click="submitOwnerMessage" 
            :disabled="!canSubmitMessage || submittingMessage"
            class="btn-primary"
          >
            {{ submittingMessage ? '處理中...' : '開始生成影片' }}
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
import { ref, computed } from 'vue'
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

// Step 3: 影片
const videos = ref([])
const videoInput = ref(null)
const uploading = ref(false)
const uploadProgress = ref(0)

// Step 4: 圖片
const endingImage = ref(null)
const imagePreview = ref('')
const imageInput = ref(null)
const uploadingImage = ref(false)

// Step 5: 留言
const ownerMessage = ref('')
const submittingMessage = ref(false)

// 專案
const projectId = ref('')
const statusMessage = ref('')
const progress = ref(0)
const result = ref(null)

// 計算屬性
const canProceedStep1 = computed(() => {
  return dogName.value.trim().length > 0 && ownerRelationship.value !== '' && storyMode.value
})

const videoCount = computed(() => videos.value.length)

const canSubmitMessage = computed(() => {
  return ownerMessage.value.trim().length >= 10
})

const createProject = async () => {
  if (!canProceedStep1.value) return
  
  try {
    const response = await axios.post('/api/v2/story/projects', {
      name: `${dogName.value}的告白`,
      dog_name: dogName.value,
      dog_breed: dogBreed.value,
      owner_relationship: ownerRelationship.value,
      story_mode: storyMode.value
    })
    
    projectId.value = response.data.project_id
    currentStep.value = 2
  } catch (error) {
    alert('建立專案失敗：' + (error.response?.data?.error || error.message))
  }
}

const handleVideoSelect = (event) => {
  const file = event.target.files[0]
  if (!file) return
  // 去重（依檔名+size）
  const alreadyAdded = videos.value.some(f => f.name === file.name && f.size === file.size)
  if (!alreadyAdded && videos.value.length < 5) {
    videos.value.push(file)
  }
  event.target.value = '' // 清空讓同一支影片可以重新選
}

const removeVideo = (index) => {
  videos.value.splice(index, 1)
}

const formatFileSize = (bytes) => {
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
}

const CHUNK_SIZE = 20 * 1024 * 1024 // 20MB

const uploadFileChunked = async (pid, file, onChunkDone) => {
  const totalChunks = Math.ceil(file.size / CHUNK_SIZE)
  const fileId = Date.now().toString(36) + Math.random().toString(36).substr(2)

  // 同時送最多 3 個 chunk，加快上傳速度
  const PARALLEL = 3
  for (let i = 0; i < totalChunks; i += PARALLEL) {
    const batch = []
    for (let j = i; j < Math.min(i + PARALLEL, totalChunks); j++) {
      const start = j * CHUNK_SIZE
      const chunk = file.slice(start, Math.min(start + CHUNK_SIZE, file.size))
      const formData = new FormData()
      formData.append('chunk', chunk)
      formData.append('file_id', fileId)
      formData.append('chunk_index', j)
      formData.append('total_chunks', totalChunks)
      formData.append('filename', file.name)
      batch.push(
        axios.post(`/api/v2/story/projects/${pid}/video-chunk`, formData, {
          headers: { 'Content-Type': 'multipart/form-data' }
        }).then(() => onChunkDone && onChunkDone())
      )
    }
    await Promise.all(batch)
  }
}

const uploadVideos = async () => {
  uploading.value = true
  uploadProgress.value = 0

  try {
    const fileList = videos.value
    // 計算總 chunk 數以追蹤整體進度
    let totalChunks = 0
    let doneChunks = 0
    for (const video of fileList) {
      totalChunks += video.size > 25 * 1024 * 1024
        ? Math.ceil(video.size / CHUNK_SIZE)
        : 1
    }

    const onChunkDone = () => {
      doneChunks++
      uploadProgress.value = Math.round((doneChunks / totalChunks) * 100)
    }

    // 所有影片同時並行上傳
    await Promise.all(fileList.map(video => {
      if (video.size > 25 * 1024 * 1024) {
        return uploadFileChunked(projectId.value, video, onChunkDone)
      } else {
        const formData = new FormData()
        formData.append('videos', video)
        return axios.post(`/api/v2/story/projects/${projectId.value}/videos`, formData, {
          headers: { 'Content-Type': 'multipart/form-data' }
        }).then(() => onChunkDone())
      }
    }))

    currentStep.value = 3
  } catch (error) {
    alert('上傳影片失敗：' + (error.response?.data?.error || error.message))
  } finally {
    uploading.value = false
    uploadProgress.value = 0
  }
}

const handleImageSelect = (event) => {
  const file = event.target.files[0]
  if (file) {
    endingImage.value = file
    imagePreview.value = URL.createObjectURL(file)
  }
}

const removeImage = () => {
  endingImage.value = null
  imagePreview.value = ''
}

const uploadImage = async () => {
  uploadingImage.value = true
  
  try {
    const formData = new FormData()
    formData.append('image', endingImage.value)
    
    await axios.post(`/api/v2/story/projects/${projectId.value}/ending-image`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
    
    currentStep.value = 4
  } catch (error) {
    alert('上傳圖片失敗：' + (error.response?.data?.error || error.message))
  } finally {
    uploadingImage.value = false
  }
}

const submitOwnerMessage = async () => {
  if (!canSubmitMessage.value) return
  
  submittingMessage.value = true
  try {
    await axios.post(`/api/v2/story/projects/${projectId.value}/owner-message`, {
      message: ownerMessage.value
    })

    await axios.post(`/api/v2/story/projects/${projectId.value}/generate`)
    
    currentStep.value = 5
    pollProgress()
  } catch (error) {
    alert('提交失敗：' + (error.response?.data?.error || error.message))
  } finally {
    submittingMessage.value = false
  }
}

const pollProgress = async () => {
  const interval = setInterval(async () => {
    try {
      const response = await axios.get(`/api/v2/story/projects/${projectId.value}`)
      const status = response.data.status
      
      if (status === 'analyzing') {
        statusMessage.value = '正在分析影片內容...'
        progress.value = response.data.progress || 25
      } else if (status === 'generating_story') {
        statusMessage.value = '正在創作對白...'
        progress.value = response.data.progress || 50
      } else if (status === 'generating_video') {
        statusMessage.value = '正在合成最終影片...'
        progress.value = response.data.progress || 75
      } else if (status === 'completed') {
        clearInterval(interval)
        progress.value = 100
        result.value = response.data
        setTimeout(() => {
          currentStep.value = 6
        }, 500)
      } else if (status === 'failed') {
        clearInterval(interval)
        alert('處理失敗：' + response.data.error)
        currentStep.value = 1
      }
    } catch (error) {
      console.error('查詢進度失敗:', error)
    }
  }, 3000)
}

const reset = () => {
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
