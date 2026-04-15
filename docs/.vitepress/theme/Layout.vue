<script setup lang="ts">
// Thin wrapper around DefaultTheme.Layout that injects blog chrome via slots.
// For posts (frontmatter.blog === 'post'), the BlogPostHeader component is
// rendered above the page body through the `doc-before` slot.
import DefaultTheme from 'vitepress/theme'
import { useData } from 'vitepress'
import BlogPostHeader from './layouts/BlogPostHeader.vue'
import BlogPostFooter from './layouts/BlogPostFooter.vue'

const { frontmatter } = useData()
const DefaultLayout = DefaultTheme.Layout
</script>

<template>
  <DefaultLayout>
    <template v-if="frontmatter.blog === 'post'" #doc-before>
      <BlogPostHeader />
    </template>
    <template v-if="frontmatter.blog === 'post'" #doc-after>
      <BlogPostFooter />
    </template>
  </DefaultLayout>
</template>
