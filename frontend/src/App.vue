<template>
  <div id="app">

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
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); /* Ensure body has background */
  overscroll-behavior-y: none; /* Disable pull-to-refresh */
}

html {
  background-color: #667eea; /* Fallback for elastic scroll */
  overscroll-behavior-y: none; /* Disable pull-to-refresh */
  height: 100%;
}

#app {
  min-height: 100vh;
}

.banner {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 1.8rem 1.5rem;
  box-shadow: 0 4px 24px rgba(0,0,0,0.15);
}

.banner-content {
  max-width: 1200px;
  margin: 0 auto;
  color: white;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.banner-title {
  font-size: 1.8rem;
  font-weight: 800;
  letter-spacing: 0.05em;
  text-shadow: 2px 2px 8px rgba(0,0,0,0.25);
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.banner-subtitle {
  font-size: 1rem;
  opacity: 0.9;
}

.paws {
  font-size: 1.6rem;
}

main {
  max-width: 1200px;
  margin: 2rem auto;
  padding: 0 1.5rem;
}

@media (max-width: 768px) {
  .banner {
    padding: 1.2rem 1rem;
  }
  .banner-title {
    font-size: 1.4rem;
  }
  .banner-subtitle {
    font-size: 0.95rem;
  }
  main {
    margin: 0;
    padding: 0;
    width: 100%;
    max-width: 100%;
  }
}
</style>
