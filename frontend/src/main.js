import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'

// Import pages - URL structure matches folder structure
import PocJobs from './pages/poc/jobs/index.vue'
import PocJobDetail from './pages/poc/jobs/[id].vue'
import LoveStory from './pages/love-story/index.vue'
import Admin from './pages/admin/index.vue'

// admin-xxx 子網域（如 admin-paw.ricky-nova.com）直接進後台；一般網域進主站。
const isAdminHost = typeof window !== 'undefined' && window.location.hostname.startsWith('admin')

const routes = [
  { path: '/', redirect: () => (isAdminHost ? '/admin' : '/love-story') },
  { path: '/poc/jobs', component: PocJobs },
  { path: '/poc/jobs/:id', component: PocJobDetail },
  { path: '/love-story', component: LoveStory },
  { path: '/admin', component: Admin }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

const app = createApp(App)
app.use(router)
app.mount('#app')
