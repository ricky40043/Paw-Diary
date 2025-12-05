<template>
  <div class="poc-jobs">
    <div class="container">
      <h1>🎬 POC - 影片分析任務</h1>
      
      <div class="upload-section">
        <h2>上傳影片</h2>
        <div class="upload-area" @dragover.prevent @drop.prevent="handleDrop">
          <input 
            type="file" 
            ref="fileInput" 
            @change="handleFileSelect" 
            accept="video/mp4,video/mov,video/avi"
            style="display: none"
          />
          <div v-if="!selectedFile" class="upload-prompt" @click="$refs.fileInput.click()">
            <div class="upload-icon">📁</div>
            <p>點擊選擇影片或拖曳檔案到此</p>
            <p class="hint">支援格式：MP4, MOV, AVI</p>
          </div>
          <div v-else class="file-selected">
            <div class="file-icon">🎥</div>
            <p class="file-name">{{ selectedFile.name }}</p>
            <p class="file-size">{{ formatFileSize(selectedFile.size) }}</p>
            <button @click="uploadFile" class="btn-upload" :disabled="uploading">
              {{ uploading ? '上傳中...' : '開始上傳' }}
            </button>
            <button @click="clearFile" class="btn-clear">清除</button>
          </div>
        </div>
      </div>

      <div class="jobs-section">
        <h2>任務列表</h2>
        <button @click="loadJobs" class="btn-refresh">🔄 重新整理</button>
        
        <div v-if="loading" class="loading">載入中...</div>
        
        <div v-else-if="jobs.length === 0" class="empty">
          <p>尚無任務</p>
        </div>
        
        <div v-else class="jobs-grid">
          <div v-for="job in jobs" :key="job.id" class="job-card">
            <div class="job-header">
              <h3>任務 #{{ job.id.substring(0, 8) }}</h3>
              <span :class="['status', job.status]">{{ getStatusText(job.status) }}</span>
            </div>
            <div class="job-info">
              <p>建立時間：{{ formatTime(job.created_at) }}</p>
              <p>更新時間：{{ formatTime(job.updated_at) }}</p>
            </div>
            <router-link :to="`/poc/jobs/${job.id}`" class="btn-view">
              查看詳情
            </router-link>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'

const router = useRouter()
const fileInput = ref(null)
const selectedFile = ref(null)
const uploading = ref(false)
const jobs = ref([])
const loading = ref(false)

const handleFileSelect = (event) => {
  const file = event.target.files[0]
  if (file) {
    selectedFile.value = file
  }
}

const handleDrop = (event) => {
  const file = event.dataTransfer.files[0]
  if (file && file.type.startsWith('video/')) {
    selectedFile.value = file
  }
}

const clearFile = () => {
  selectedFile.value = null
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

const formatFileSize = (bytes) => {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
}

const uploadFile = async () => {
  if (!selectedFile.value) return
  
  uploading.value = true
  const formData = new FormData()
  formData.append('file', selectedFile.value)
  
  try {
    const response = await axios.post('/api/v1/poc/jobs', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
    
    alert('上傳成功！正在處理影片...')
    clearFile()
    
    // Navigate to job detail
    router.push(`/poc/jobs/${response.data.job_id}`)
  } catch (error) {
    alert('上傳失敗：' + (error.response?.data?.error || error.message))
  } finally {
    uploading.value = false
  }
}

const loadJobs = async () => {
  loading.value = true
  try {
    const response = await axios.get('/api/v1/poc/jobs')
    jobs.value = response.data.jobs || []
  } catch (error) {
    console.error('Failed to load jobs:', error)
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
  loadJobs()
})
</script>

<style scoped>
.container {
  background: rgba(255, 255, 255, 0.95);
  padding: 2rem;
  border-radius: 15px;
  box-shadow: 0 10px 30px rgba(0,0,0,0.2);
}

h1 {
  color: #667eea;
  margin-bottom: 2rem;
}

h2 {
  color: #667eea;
  margin: 2rem 0 1rem 0;
}

.upload-section {
  margin-bottom: 3rem;
}

.upload-area {
  border: 3px dashed #667eea;
  border-radius: 15px;
  padding: 3rem;
  text-align: center;
  background: #f8f9ff;
  cursor: pointer;
  transition: all 0.3s;
}

.upload-area:hover {
  background: #f0f2ff;
  border-color: #764ba2;
}

.upload-prompt {
  color: #667eea;
}

.upload-icon, .file-icon {
  font-size: 4rem;
  margin-bottom: 1rem;
}

.hint {
  color: #999;
  font-size: 0.9rem;
  margin-top: 0.5rem;
}

.file-selected {
  color: #333;
}

.file-name {
  font-size: 1.2rem;
  font-weight: 600;
  margin: 1rem 0 0.5rem 0;
}

.file-size {
  color: #666;
  margin-bottom: 1.5rem;
}

.btn-upload, .btn-clear, .btn-refresh, .btn-view {
  padding: 0.8rem 1.5rem;
  border: none;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s;
  margin: 0 0.5rem;
}

.btn-upload {
  background: #667eea;
  color: white;
}

.btn-upload:hover:not(:disabled) {
  background: #5568d3;
}

.btn-upload:disabled {
  background: #ccc;
  cursor: not-allowed;
}

.btn-clear {
  background: #f44336;
  color: white;
}

.btn-clear:hover {
  background: #da190b;
}

.btn-refresh {
  background: #4caf50;
  color: white;
  margin-bottom: 1rem;
}

.btn-refresh:hover {
  background: #45a049;
}

.btn-view {
  background: #667eea;
  color: white;
  text-decoration: none;
  display: inline-block;
  margin-top: 1rem;
}

.btn-view:hover {
  background: #5568d3;
}

.jobs-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.job-card {
  background: white;
  border: 2px solid #e0e0e0;
  border-radius: 10px;
  padding: 1.5rem;
  transition: all 0.3s;
}

.job-card:hover {
  border-color: #667eea;
  transform: translateY(-5px);
  box-shadow: 0 5px 15px rgba(102, 126, 234, 0.2);
}

.job-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.job-header h3 {
  color: #333;
  font-size: 1.1rem;
}

.status {
  padding: 0.3rem 0.8rem;
  border-radius: 20px;
  font-size: 0.85rem;
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

.job-info p {
  color: #666;
  font-size: 0.9rem;
  margin: 0.3rem 0;
}

.loading, .empty {
  text-align: center;
  padding: 3rem;
  color: #999;
  font-size: 1.1rem;
}
</style>
