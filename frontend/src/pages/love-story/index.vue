<template>
  <div class="love-story">
    <div class="container">
      <h1>💝 給主人的告白</h1>
      <p class="subtitle">上傳 5 個狗狗影片，讓 AI 創作感人的愛的告白</p>

      <!-- Step 1: 狗狗資訊 -->
      <div v-if="currentStep === 1" class="step-card">
        <h2>步驟 1：狗狗資訊</h2>
        <form @submit.prevent="createProject">
          <div class="form-group">
            <label>狗狗名字 *</label>
            <input v-model="dogName" type="text" placeholder="例如：豆豆" required />
          </div>
          <div class="form-group">
            <label>品種（可選）</label>
            <input v-model="dogBreed" type="text" placeholder="例如：吉娃娃" />
          </div>
          <button type="submit" class="btn-primary">下一步</button>
        </form>
      </div>

      <!-- Step 2: 上傳 5 個影片 -->
      <div v-if="currentStep === 2" class="step-card">
        <h2>步驟 2：上傳 5 個影片</h2>
        <p class="hint">每個影片會被剪輯成約 15 秒，請選擇與狗狗互動的溫馨片段</p>
        
        <div class="video-uploads">
          <div v-for="i in 5" :key="i" class="video-upload-box">
            <div v-if="!videos[i-1]" class="upload-placeholder" @click="selectVideo(i-1)">
              <div class="icon">🎬</div>
              <p>影片 {{ i }}</p>
              <p class="small">點擊選擇</p>
            </div>
            <div v-else class="video-selected">
              <div class="icon">✅</div>
              <p class="name">{{ videos[i-1].name }}</p>
              <p class="size">{{ formatFileSize(videos[i-1].size) }}</p>
              <button @click="removeVideo(i-1)" class="btn-remove">移除</button>
            </div>
          </div>
        </div>
        <input 
          ref="videoInput" 
          type="file" 
          accept="video/mp4,video/mov,video/avi" 
          @change="handleVideoSelect" 
          style="display: none"
        />
        
        <div class="actions">
          <button @click="currentStep = 1" class="btn-secondary">上一步</button>
          <button 
            @click="uploadVideos" 
            :disabled="videos.filter(v => v).length < 5 || uploading"
            class="btn-primary"
          >
            {{ uploading ? '上傳中...' : '上傳影片' }}
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
            <button @click="removeImage" class="btn-remove">更換圖片</button>
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
        <h2>步驟 4：給狗狗的話</h2>
        <p class="hint">寫下你想對狗狗說的話，這將會成為影片中感人的一幕</p>
        
        <div class="form-group">
          <label>給寶貝的一句話 *</label>
          <textarea 
            v-model="ownerMessage" 
            rows="4" 
            placeholder="例如：謝謝你來到我的生命中，你是我最好的朋友..." 
            class="message-input"
            required
          ></textarea>
        </div>
        
        <div class="actions">
          <button @click="currentStep = 3" class="btn-secondary">上一步</button>
          <button 
            @click="submitOwnerMessage" 
            :disabled="!ownerMessage.trim() || submittingMessage"
            class="btn-primary"
          >
            {{ submittingMessage ? '處理中...' : '開始生成影片' }}
          </button>
        </div>
      </div>

      <!-- Step 5: 處理中 -->
      <div v-if="currentStep === 5" class="step-card processing">
        <div class="spinner"></div>
        <h2>✨ AI 正在創作中...</h2>
        <p>{{ statusMessage }}</p>
        <div class="progress">
          <div class="progress-bar" :style="{width: progress + '%'}"></div>
        </div>
      </div>

      <!-- Step 6: 完成 -->
      <div v-if="currentStep === 6" class="step-card completed">
        <h2>🎉 完成！</h2>
        <div v-if="result" class="result">
          <h3>{{ result.story.title }}</h3>
          <div class="chapters">
            <div v-for="(chapter, index) in result.story.chapters" :key="index" class="chapter">
              <span class="index">{{ index + 1 }}</span>
              <p>{{ chapter.narration }}</p>
            </div>
          </div>
          <div class="final-message">
            <p>💝 {{ result.story.final_message }}</p>
          </div>
          
          <div class="video-player">
            <video controls :src="result.final_video_url">
              您的瀏覽器不支援影片播放
            </video>
          </div>

          <div class="actions">
            <a :href="result.final_video_url" download class="btn-primary">
              ⬇️ 下載影片
            </a>
            <button @click="reset" class="btn-secondary">建立新的告白</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import axios from 'axios'

const currentStep = ref(1)
const dogName = ref('')
const dogBreed = ref('')
const dogBreed = ref('')
const projectId = ref('')
const ownerMessage = ref('')
const submittingMessage = ref(false)
const videos = ref([null, null, null, null, null])
const selectedVideoIndex = ref(-1)
const videoInput = ref(null)
const uploading = ref(false)
const endingImage = ref(null)
const imagePreview = ref('')
const imageInput = ref(null)
const uploadingImage = ref(false)
const statusMessage = ref('')
const progress = ref(0)
const result = ref(null)

const createProject = async () => {
  try {
    const response = await axios.post('/api/v2/story/projects', {
      name: `${dogName.value}的告白`,
      dog_name: dogName.value,
      dog_breed: dogBreed.value
    })
    
    projectId.value = response.data.project_id
    currentStep.value = 2
  } catch (error) {
    alert('建立專案失敗：' + (error.response?.data?.error || error.message))
  }
}

const selectVideo = (index) => {
  selectedVideoIndex.value = index
  videoInput.value.click()
}

const handleVideoSelect = (event) => {
  const file = event.target.files[0]
  if (file && selectedVideoIndex.value >= 0) {
    videos.value[selectedVideoIndex.value] = file
  }
  event.target.value = ''
}

const removeVideo = (index) => {
  videos.value[index] = null
}

const formatFileSize = (bytes) => {
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
}

const uploadVideos = async () => {
  uploading.value = true
  const formData = new FormData()
  
  videos.value.forEach(video => {
    if (video) {
      formData.append('videos', video)
    }
  })
  
  try {
    await axios.post(`/api/v2/story/projects/${projectId.value}/videos`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
    
    currentStep.value = 3
  } catch (error) {
    alert('上傳影片失敗：' + (error.response?.data?.error || error.message))
  } finally {
    uploading.value = false
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
    // 上傳結尾圖片
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
  submittingMessage.value = true
  try {
    // 1. 設定主人留言
    await axios.post(`/api/v2/story/projects/${projectId.value}/owner-message`, {
      message: ownerMessage.value
    })

    // 2. 開始生成
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
        progress.value = 25
      } else if (status === 'generating_story') {
        statusMessage.value = '正在創作感人對白...'
        progress.value = 50
      } else if (status === 'generating_video') {
        statusMessage.value = '正在合成最終影片...'
        progress.value = 75
      } else if (status === 'completed') {
        clearInterval(interval)
        progress.value = 100
        result.value = response.data
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
  dogName.value = ''
  dogBreed.value = ''
  projectId.value = ''
  ownerMessage.value = ''
  videos.value = [null, null, null, null, null]
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

.step-card {
  background: white;
  border-radius: 20px;
  padding: 2.5rem;
  box-shadow: 0 10px 40px rgba(0,0,0,0.2);
}

h2 {
  color: #667eea;
  margin-bottom: 1.5rem;
}

.hint {
  color: #666;
  margin-bottom: 1.5rem;
}

.form-group {
  margin-bottom: 1.5rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 600;
  color: #333;
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

.message-input {
  width: 100%;
  padding: 0.8rem;
  border: 2px solid #e0e0e0;
  border-radius: 8px;
  font-size: 1rem;
  resize: vertical;
  font-family: inherit;
}

.video-uploads {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 1rem;
  margin-bottom: 2rem;
}

.video-upload-box {
  aspect-ratio: 1;
  border: 3px dashed #667eea;
  border-radius: 15px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.3s;
}

.video-upload-box:hover {
  background: #f8f9ff;
  border-color: #764ba2;
}

.upload-placeholder, .video-selected {
  text-align: center;
  padding: 1rem;
}

.icon {
  font-size: 3rem;
  margin-bottom: 0.5rem;
}

.video-selected .name {
  font-weight: 600;
  color: #333;
  margin: 0.5rem 0;
  font-size: 0.9rem;
  word-break: break-all;
}

.video-selected .size {
  color: #666;
  font-size: 0.8rem;
}

.small {
  color: #999;
  font-size: 0.85rem;
}

.image-upload {
  max-width: 500px;
  margin: 0 auto 2rem auto;
}

.image-preview img {
  width: 100%;
  border-radius: 15px;
  margin-bottom: 1rem;
}

.actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
}

.btn-primary, .btn-secondary, .btn-remove {
  padding: 0.8rem 2rem;
  border: none;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s;
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
  font-size: 0.85rem;
  padding: 0.5rem 1rem;
}

.btn-remove:hover {
  background: #da190b;
}

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
  font-size: 1.3rem;
  font-weight: 600;
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
