import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import Layout from './Layout.vue'
import BlogIndex from './layouts/BlogIndex.vue'
import './custom.css'

// Register BlogIndex as a global component so blog/index.md can invoke it
// inline (see `<BlogIndex />` in that file).
export default {
  extends: DefaultTheme,
  Layout,
  enhanceApp({ app }) {
    app.component('BlogIndex', BlogIndex)
  },
} satisfies Theme
