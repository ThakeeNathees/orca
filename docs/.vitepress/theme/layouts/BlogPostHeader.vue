<script setup lang="ts">
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

// Renders above the post body via the DefaultTheme `doc-before` slot.
// Pulls title / date / category / readTime from the page's frontmatter —
// readTime is computed at build time by the transformPageData hook in config.ts.

const { frontmatter } = useData()

const dateFormatted = computed(() => {
  const d = frontmatter.value.date
  if (!d) return ''
  // Short format — same shape as the index row so the two pages agree.
  return new Date(d).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
})
</script>

<template>
  <header class="blog-post-header">
    <div class="blog-post-crumb">
      Blog &middot; {{ frontmatter.category || 'Personal' }}
    </div>
    <h1 class="blog-post-title">{{ frontmatter.title }}</h1>
    <div class="blog-post-byline">
      <img
        class="blog-post-avatar"
        :src="withBase('/author.png')"
        alt="Thakee Nathees"
      />
      <div class="blog-post-byline-text">
        <div class="blog-post-author">Thakee Nathees</div>
        <div class="blog-post-meta">
          <time v-if="dateFormatted">{{ dateFormatted }}</time>
          <span class="blog-post-sep">&middot;</span>
          <span>{{ frontmatter.readTime || 3 }} min read</span>
        </div>
      </div>
    </div>
  </header>
</template>
