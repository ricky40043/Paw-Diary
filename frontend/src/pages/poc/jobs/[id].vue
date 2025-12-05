<template>
  <div class="job-detail">
    <div class="container">
      <button @click="$router.back()" class="btn-back">← 返回列表</button>
      
      <div v-if="loading" class="loading">載入中...</div>
      
      <div v-else-if="job" class="job-content">
        <div class="header">
          <h1>任務詳情</h1>
          <span :class="['status', job.status]">{{ getStatusText(job.status) }}</span>
        </div>

        <div class="info-card">
          <h2>基本資訊</h2>
          <div class="info-grid">
            <div class="info-item">
              <label>任務 ID：</label>
              <span>{{ job.id }}</span>
            </div>
            <div class="info-item">
              <label>狀態：</label>
              <span>{{ getStatusText(job.status) }}</span>
            </div>
            <div class="info-item">
              <label>建立時間：</label>
              <span>{{ formatTime(job.created_at) }}</span>
            </div>
            <div class="info-item">
              <label>更新時間：</label>
              <span>{{ formatTime(job.updated_at) }}</span>
            </div>
          </div>
        </div>

        <div v-if="job.error" class="error-card">
          <h2>❌ 錯誤訊息</h2>
          <p>{{ job.error }}</p>
        </div>

        <div v-if="job.status === 'processing'" class="processing-card">
          <div class="spinner"></div>
          <h2>⚙️ 處理中...</h2>
          <p>AI 正在分析您的影片，請稍候</p>
        </div>

        <div v-if="job.status === 'completed' && job.highlights" class="highlights-section">
          <h2>✨ 精彩片段</h2>
          
          <div v-if="job.highlights.length === 0" class="no-highlights">
            <p>未找到明顯的互動片段</p>
          </div>
          
          <div v-else>
            <div class="highlights-list">
              <div v-for="(highlight, index) in job.highlights" :key="index" class="highlight-card">
                <h3>片段 {{ index + 1 }}</h3>
                <div class="highlight-info">
                  <p><strong>時間：</strong>{{ highlight.start.toFixed(2) }}s - {{ highlight.end.toFixed(2) }}s</p>
                  <p><strong>時長：</strong>{{ (highlight.end - highlight.start).toFixed(2) }}s</p>
                  <p><strong>互動類型：</strong>{{ highlight.interaction }}</p>
                  <p><strong>情緒：</strong>{{ highlight.emotion }}</p>
                  <p><strong>描述：</strong>{{ highlight.caption }}</p>
                </div>
              </div>
            </div>

            <div v-if="job.highlight_video_url" class="video-section">
              <h2>🎬 精華影片</h2>
              <video controls :src="job.highlight_video_url" class="highlight-video">
                您的瀏覽器不支援影片播放
              </video>
              <a :href="job.highlight_video_url" download class="btn-download">
                ⬇️ 下載影片
              </a>
            </div>
          </div>
        </div>
      </div>

      <div v-else class="error">
        找不到該任務
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'

const route = useRoute()
const job = ref(null)
const loading = ref(true)
let pollInterval = null

const loadJob = async () => {
  try {
    const response = await axios.get(`/api/v1/poc/jobs/${route.params.id}`)
    job.value = response.data
    
    // Stop polling if job is completed or failed
    if (job.value.status === 'completed' || job.value.status === 'failed') {
      if (pollInterval) {
        clearInterval(pollInterval)
        pollInterval = null
      }
    }
  } catch (error) {
    console.error('Failed to load job:', error)
  } finally {
    loading.value = false
  }
}

const getStatusText = (status) => {
  const statusMap = {
    'pending': '等待中',
    'processing': '處理中',
    'completed': '完成',
    'failed': '失敗'
  }
  return statusMap[status] || status
}

const formatTime = (timeStr) => {
  if (!timeStr) return '-'
  return new Date(timeStr).toLocaleString('zh-TW')
}

onMounted(() => {
  loadJob()
  
  // Poll every 2 seconds if processing
  pollInterval = setInterval(() => {
    if (job.value && (job.value.status === 'pending' || job.value.status === 'processing')) {
      loadJob()
    }
  }, 2000)
})

onUnmounted(() => {
  if (pollInterval) {
    clearInterval(pollInterval)
  }
})
</script>

<style scoped>
.container {
  background: rgba(255, 255, 255, 0.95);
  padding: 2rem;
  border-radius: 15px;
  box-shadow: 0 10px 30px rgba(0,0,0,0.2);
}

.btn-back {
  background: #6c757d;
  color: white;
  border: none;
  padding: 0.8rem 1.5rem;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  margin-bottom: 1.5rem;
  transition: all 0.3s;
}

.btn-back:hover {
  background: #5a6268;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

h1 {
  color: #667eea;
  margin: 0;
}

h2 {
  color: #667eea;
  margin-bottom: 1rem;
}

.status {
  padding: 0.5rem 1rem;
  border-radius: 20px;
  font-size: 0.9rem;
  font-weight: 600;
}

.status.pending {
  background: #fff3cd;
  color: #856404;
}

.status.processing {
  background: #cfe2ff;
  color: #084298;
}

.status.completed {
  background: #d1e7dd;
  color: #0f5132;
}

.status.failed {
  background: #f8d7da;
  color: #842029;
}

.info-card, .error-card, .processing-card, .highlights-section {
  background: white;
  border: 2px solid #e0e0e0;
  border-radius: 10px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1rem;
}

.info-item {
  display: flex;
  flex-direction: column;
}

.info-item label {
  font-weight: 600;
  color: #666;
  margin-bottom: 0.3rem;
}

.info-item span {
  color: #333;
}

.error-card {
  border-color: #f44336;
  background: #ffebee;
}

.error-card h2 {
  color: #c62828;
}

.processing-card {
  text-align: center;
  padding: 3rem;
}

.spinner {
  width: 50px;
  height: 50px;
  border: 5px solid #f3f3f3;
  border-top: 5px solid #667eea;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto 1rem auto;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.highlights-list {
  display: grid;
  gap: 1rem;
  margin-bottom: 2rem;
}

.highlight-card {
  background: #f8f9ff;
  border: 2px solid #667eea;
  border-radius: 10px;
  padding: 1.5rem;
}

.highlight-card h3 {
  color: #667eea;
  margin-bottom: 1rem;
}

.highlight-info p {
  margin: 0.5rem 0;
  color: #333;
}

.video-section {
  text-align: center;
  margin-top: 2rem;
}

.highlight-video {
  max-width: 100%;
  width: 800px;
  border-radius: 10px;
  box-shadow: 0 5px 15px rgba(0,0,0,0.2);
  margin: 1rem 0;
}

.btn-download {
  display: inline-block;
  background: #4caf50;
  color: white;
  padding: 1rem 2rem;
  border-radius: 8px;
  text-decoration: none;
  font-weight: 600;
  margin-top: 1rem;
  transition: all 0.3s;
}

.btn-download:hover {
  background: #45a049;
  transform: scale(1.05);
}

.no-highlights {
  text-align: center;
  padding: 2rem;
  color: #999;
}

.loading, .error {
  text-align: center;
  padding: 3rem;
  color: #999;
  font-size: 1.1rem;
}
</style>
