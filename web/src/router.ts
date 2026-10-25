import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'

// 登录不再是一个独立页面，而是首页上的浮层；/login 一律送回首页
const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: Home },
    { path: '/login', redirect: '/' },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

export default router
