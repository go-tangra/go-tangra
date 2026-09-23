import { createApp } from 'vue'
import { createRouter, createWebHashHistory } from 'vue-router'
import App from './App.vue'
import { pages } from './pages'
import './main.css'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [{ path: '/', redirect: '/' + pages[0]!.key }, ...pages.map((p) => ({ path: '/' + p.key, name: p.key, component: p.component }))],
})
createApp(App).use(router).mount('#app')
