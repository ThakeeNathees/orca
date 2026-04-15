<script setup lang="ts">
import { ref, computed } from 'vue'
import { withBase } from 'vitepress'
import { data as posts } from '../../../blog/posts.data.ts'

// Keep the category list hand-maintained — expand as new categories appear.
const CATEGORIES = ['All', 'Language', 'Release', 'Personal'] as const
type Category = typeof CATEGORIES[number]

const active = ref<Category>('All')

const filtered = computed(() =>
  active.value === 'All' ? posts : posts.filter((p) => p.category === active.value)
)
</script>

<template>
  <div class="blog">
    <header class="blog-header">
      <h1 class="blog-title">Blog</h1>
      <p class="blog-tagline">
        Notes on building Orca &mdash; a declarative language for AI agents.
      </p>
    </header>

    <nav class="blog-tabs" aria-label="Categories">
      <button
        v-for="c in CATEGORIES"
        :key="c"
        class="blog-tab"
        :class="{ active: active === c }"
        @click="active = c"
      >
        {{ c }}
      </button>
    </nav>

    <ul v-if="filtered.length" class="blog-rows">
      <li v-for="p in filtered" :key="p.url" class="blog-row-item">
        <a :href="withBase(p.url)" class="blog-row">
          <span class="blog-row-meta">
            <span class="blog-row-date">{{ p.dateFormatted }}</span>
            <span class="blog-row-sep">&middot;</span>
            <span class="blog-row-category">{{ p.category }}</span>
          </span>
          <span class="blog-row-title">{{ p.title }}</span>
          <span class="blog-row-author">Thakee Nathees</span>
          <span class="blog-row-read">{{ p.readTime }}m</span>
        </a>
      </li>
    </ul>
    <p v-else class="blog-empty">No posts in this category yet.</p>
  </div>
</template>
