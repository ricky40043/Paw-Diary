<template>
  <div id="app">
    <header>
      <nav>
        <router-link to="/">首頁</router-link>
        <router-link to="/love-story">給主人的告白</router-link>
        <router-link to="/poc/jobs">POC 任務</router-link>
      </nav>
    </header>
    <main>
      <router-view />
    </main>
  </div>
</template>

<script setup>
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Microsoft JhengHei', sans-serif;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  min-height: 100vh;
}

#app {
  min-height: 100vh;
}

header {
  background: rgba(255, 255, 255, 0.95);
  padding: 1rem 2rem;
  box-shadow: 0 2px 10px rgba(0,0,0,0.1);
}

nav {
  display: flex;
  gap: 2rem;
  max-width: 1200px;
  margin: 0 auto;
}

nav a {
  color: #667eea;
  text-decoration: none;
  font-weight: 600;
  padding: 0.5rem 1rem;
  border-radius: 8px;
  transition: all 0.3s;
}

nav a:hover {
  background: #667eea;
  color: white;
}

nav a.router-link-active {
  background: #667eea;
  color: white;
}

main {
  max-width: 1200px;
  margin: 2rem auto;
  padding: 0 2rem;
}
</style>
