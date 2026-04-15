// Build-time data loader for blog posts.
// Enumerates every `docs/blog/posts/*.md` file, parses frontmatter, computes
// reading time from word count, and pulls the first image (if any) as a cover.
// Consumers import via `import { data as posts } from '../../../blog/posts.data.ts'`.

import { createContentLoader } from 'vitepress'

export interface Post {
  url: string
  title: string
  description?: string
  date: string           // ISO from frontmatter
  dateFormatted: string  // "Apr 15, 2026"
  dateMs: number         // for sort
  category: string
  cover?: string         // first image URL, if any
  readTime: number       // minutes
}

export declare const data: Post[]

export default createContentLoader('blog/posts/*.md', {
  includeSrc: true,
  transform(raw): Post[] {
    return raw
      .map(({ url, frontmatter, src }) => {
        const body = (src || '')
          .replace(/^---[\s\S]*?---/, '')        // strip frontmatter
          .replace(/```[\s\S]*?```/g, '')         // strip code blocks
          .replace(/!\[[^\]]*\]\([^)]+\)/g, '')   // strip image markup for word count
          .replace(/\[([^\]]*)\]\([^)]+\)/g, '$1')// keep link text
          .replace(/[#*_>`~]/g, '')               // strip markdown punct

        const words = body.split(/\s+/).filter(Boolean).length
        const readTime = Math.max(1, Math.round(words / 200))

        const coverMatch = (src || '').match(/!\[[^\]]*\]\(([^)]+)\)/)
        const cover = coverMatch?.[1]

        const d = new Date(frontmatter.date)
        const dateFormatted = d.toLocaleDateString('en-US', {
          month: 'short',
          day: 'numeric',
          year: 'numeric',
        })

        return {
          url,
          title: frontmatter.title ?? url,
          description: frontmatter.description,
          date: frontmatter.date,
          dateFormatted,
          dateMs: +d,
          category: frontmatter.category ?? 'Personal',
          cover,
          readTime,
        }
      })
      .sort((a, b) => b.dateMs - a.dateMs)
  },
})
