<script setup lang="ts">
import { withBase } from 'vitepress'
</script>

<template>
  <footer class="blog-post-footer">
    <hr class="blog-post-rule" />
    <a :href="withBase('/blog/')" class="blog-post-more">
      View more posts &rarr;
    </a>
  </footer>
</template>
