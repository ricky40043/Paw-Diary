<template>
  <div class="home">
    <div class="hero">
      <h1>🐕 狗狗回憶影片自動剪輯</h1>
      <p class="subtitle">上傳影片，AI 自動找出你與狗狗的互動精彩時刻</p>
      
      <div class="features">
        <div class="feature-card">
          <div class="icon">📹</div>
          <h3>自動分析</h3>
          <p>AI 自動分析影片中的互動片段</p>
        </div>
        <div class="feature-card">
          <div class="icon">✨</div>
          <h3>智能剪輯</h3>
          <p>找出最精彩的高光時刻</p>
        </div>
        <div class="feature-card">
          <div class="icon">💝</div>
          <h3>永久珍藏</h3>
          <p>保存與狗狗的珍貴回憶</p>
        </div>
      </div>

      <div class="cta">
        <router-link to="/poc/jobs" class="btn-primary">開始使用</router-link>
      </div>
    </div>

    <div class="info-section">
      <h2>Phase 1 - 概念驗證</h2>
      <p>目前支援：</p>
      <ul>
        <li>✅ 上傳單一影片（MP4/MOV/AVI）</li>
        <li>✅ 自動抽取畫面幀</li>
        <li>✅ AI 分析互動片段</li>
        <li>✅ 生成高光精華影片</li>
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
