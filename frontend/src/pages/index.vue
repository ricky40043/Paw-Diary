<template>
  <div class="home">
    <div class="hero">
      <h1>🐾 狗狗愛的回憶</h1>
      <p class="subtitle">珍藏與毛孩相處的每一個溫馨時刻</p>
      
      <div class="features">
        <div class="feature-card">
          <div class="icon">📹</div>
          <h3>智能剪輯</h3>
          <p>自動整理影片中的精彩片段</p>
        </div>
        <div class="feature-card">
          <div class="icon">💝</div>
          <h3>溫馨故事</h3>
          <p>為你和毛孩創作專屬回憶故事</p>
        </div>
        <div class="feature-card">
          <div class="icon">🎵</div>
          <h3>永久珍藏</h3>
          <p>配上音樂和旁白，保存珍貴回憶</p>
        </div>
      </div>

      <div class="cta">
        <router-link to="/love-story" class="btn-primary">開始製作</router-link>
      </div>
    </div>

    <div class="info-section">
      <h2>如何使用</h2>
      <ul>
        <li>📱 上傳 5 個與狗狗的日常影片</li>
        <li>📝 填寫狗狗的基本資料</li>
        <li>💌 寫下想對狗狗說的話</li>
        <li>🎬 系統自動生成溫馨回憶影片</li>
      </ul>
    </div>
  </div>
</template>

<script setup>
</script>

<style scoped>
.home {
  color: white;
}

.hero {
  text-align: center;
  padding: 3rem 0;
}

h1 {
  font-size: 3rem;
  margin-bottom: 1rem;
  text-shadow: 2px 2px 4px rgba(0,0,0,0.2);
}

.subtitle {
  font-size: 1.3rem;
  margin-bottom: 3rem;
  opacity: 0.9;
}

.features {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 2rem;
  margin: 3rem 0;
}

.feature-card {
  background: rgba(255, 255, 255, 0.95);
  padding: 2rem;
  border-radius: 15px;
  box-shadow: 0 10px 30px rgba(0,0,0,0.2);
  color: #333;
  transition: transform 0.3s;
}

.feature-card:hover {
  transform: translateY(-10px);
}

.icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.feature-card h3 {
  font-size: 1.5rem;
  margin-bottom: 0.5rem;
  color: #667eea;
}

.cta {
  margin: 3rem 0;
}

.btn-primary {
  display: inline-block;
  padding: 1rem 3rem;
  background: white;
  color: #667eea;
  text-decoration: none;
  font-size: 1.2rem;
  font-weight: 700;
  border-radius: 50px;
  box-shadow: 0 5px 20px rgba(0,0,0,0.2);
  transition: all 0.3s;
}

.btn-primary:hover {
  transform: scale(1.05);
  box-shadow: 0 8px 30px rgba(0,0,0,0.3);
}

.info-section {
  background: rgba(255, 255, 255, 0.95);
  padding: 2rem;
  border-radius: 15px;
  margin-top: 3rem;
  color: #333;
}

.info-section h2 {
  color: #667eea;
  margin-bottom: 1rem;
}

.info-section ul {
  list-style: none;
  padding-left: 0;
}

.info-section li {
  padding: 0.5rem 0;
  font-size: 1.1rem;
}
</style>
