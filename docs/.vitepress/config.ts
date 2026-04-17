import { defineConfig } from 'vitepress'
import fs from 'node:fs'
import path from 'node:path'

// In dev the Studio is served separately on :3000; in prod it's bundled under /orca/studio/.
const playgroundLink = process.env.NODE_ENV === 'production' ? '/studio/' : 'http://localhost:3000'

// Absolute site URL — required for og:image / og:url so social crawlers
// (Twitter, Facebook, LinkedIn, Slack, Discord) can fetch the preview.
// Path-relative URLs do not work in OG meta tags.
const SITE_URL = 'https://thakeenathees.github.io/orca'

// Grouped sidebar used for both /reference/ and /examples/ — single source of truth.
function docsSidebar() {
  return [
    {
      text: 'Overview',
      items: [
        { text: 'Introduction',    link: '/guide/introduction' },
        { text: 'Getting Started', link: '/guide/getting-started' },
        { text: 'CLI Reference',   link: '/guide/cli' },
        { text: 'Syntax Overview', link: '/reference/syntax-overview' },
      ],
    },
    {
      text: 'Examples',
      items: [
        { text: 'Simple Agent', link: '/examples/simple-agent' },
        { text: 'Multi-Agent Workflow', link: '/examples/multi-agent-workflow' },
      ],
    },
    {
      text: 'Language Features',
      items: [
        { text: 'Constant Folding', link: '/reference/features/constant-folding' },
        { text: 'Lambdas & Closures', link: '/reference/features/lambda-closures' },
        { text: 'Compile-Time Analysis', link: '/reference/features/analyzer' },
      ],
    },
    {
      text: 'Blocks',
      items: [
        { text: 'model', link: '/reference/model' },
        { text: 'agent', link: '/reference/agent' },
        { text: 'tool', link: '/reference/tool' },
        { text: 'workflow', link: '/reference/workflow' },
        { text: 'cron', link: '/reference/cron' },
        { text: 'webhook', link: '/reference/webhook' },
        { text: 'input', link: '/reference/input' },
        { text: 'schema', link: '/reference/schema' },
        { text: 'let', link: '/reference/let' },
      ],
    },
  ]
}

export default defineConfig({
  // Build-time page-data hooks:
  //   1. Rewrite the __PLAYGROUND_URL__ placeholder in hero actions
  //   2. For blog posts, compute reading time and inject OG/Twitter meta tags
  //      (cover image = first image in the post, falling back to the favicon)
  transformPageData(pageData, ctx) {
    const actions = (pageData.frontmatter as any)?.hero?.actions
    if (Array.isArray(actions)) {
      for (const a of actions) {
        if (a?.link === '__PLAYGROUND_URL__') a.link = playgroundLink
      }
    }

    if (pageData.relativePath.startsWith('blog/posts/')) {
      const filePath = path.join(ctx.siteConfig.srcDir, pageData.relativePath)
      const src = fs.readFileSync(filePath, 'utf-8')

      // Reading time — strip frontmatter, code blocks, and markdown punctuation
      // so the word count reflects actual prose.
      const body = src
        .replace(/^---[\s\S]*?---/, '')
        .replace(/```[\s\S]*?```/g, '')
        .replace(/!\[[^\]]*\]\([^)]+\)/g, '')
        .replace(/\[([^\]]*)\]\([^)]+\)/g, '$1')
        .replace(/[#*_>`~]/g, '')
      const words = body.split(/\s+/).filter(Boolean).length
      ;(pageData.frontmatter as any).readTime = Math.max(1, Math.round(words / 200))

      // First image in the post → OG card. Must be an absolute URL — social
      // crawlers do not resolve relative paths. Falls back to the site-wide
      // OG image bundled in /public.
      const coverMatch = src.match(/!\[[^\]]*\]\(([^)]+)\)/)
      const rawCover = coverMatch?.[1] || '/og-default.png'
      const cover = rawCover.startsWith('http')
        ? rawCover
        : SITE_URL + (rawCover.startsWith('/') ? rawCover : '/' + rawCover)

      const pageUrl =
        SITE_URL + '/' + pageData.relativePath.replace(/\.md$/, '.html')

      const title = pageData.frontmatter.title || 'Orca'
      const description = pageData.frontmatter.description || ''

      pageData.frontmatter.head ??= []
      pageData.frontmatter.head.push(
        ['meta', { property: 'og:type', content: 'article' }],
        ['meta', { property: 'og:url', content: pageUrl }],
        ['meta', { property: 'og:site_name', content: 'Orca' }],
        ['meta', { property: 'og:title', content: title }],
        ['meta', { property: 'og:description', content: description }],
        ['meta', { property: 'og:image', content: cover }],
        ['meta', { property: 'og:image:alt', content: title }],
        ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
        ['meta', { name: 'twitter:title', content: title }],
        ['meta', { name: 'twitter:description', content: description }],
        ['meta', { name: 'twitter:image', content: cover }],
      )
    }
  },
  title: 'Orca',
  description: 'A declarative language for defining AI agents',
  base: '/orca/',

  // DESIGN.md lives alongside the docs as a reference for contributors but is
  // not rendered as a site page.
  srcExclude: ['DESIGN.md'],

  markdown: {
    lineNumbers: true,
    languageAlias: {
      'orca': 'hcl',
    },
  },

  head: [
    ['link', { rel: 'icon', type: 'image/png', href: '/orca/logo-white.png' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],

    // Site-wide default Open Graph / Twitter Card. Per-blog-post pages
    // override og:image with their first content image via transformPageData.
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:site_name', content: 'Orca' }],
    ['meta', { property: 'og:title', content: 'Orca' }],
    ['meta', { property: 'og:description', content: 'A declarative language for AI agents — define agents, tools, and workflows in a concise HCL-like syntax. Compiles to Python.' }],
    ['meta', { property: 'og:url', content: SITE_URL + '/' }],
    ['meta', { property: 'og:image', content: SITE_URL + '/og-default.png' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:title', content: 'Orca' }],
    ['meta', { name: 'twitter:description', content: 'A declarative language for AI agents — compiles to Python.' }],
    ['meta', { name: 'twitter:image', content: SITE_URL + '/og-default.png' }],
  ],

  themeConfig: {
    nav: [
      { text: 'Docs', link: '/guide/introduction' },
      { text: 'Blog', link: '/blog/' },
      { text: 'Playground', link: playgroundLink, target: '_blank' },
    ],

    // Single unified docs sidebar — same groups regardless of which URL prefix
    // the reader is currently on. /guide/ is kept for backwards compatibility.
    sidebar: {
      '/reference/': docsSidebar(),
      '/examples/':  docsSidebar(),
      '/guide/':     docsSidebar(),
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/ThakeeNathees/orca' },
    ],

    search: {
      provider: 'local',
    },

    footer: {
      message: 'Released under the MIT License.',
    },

    docFooter: {
      prev: true,
      next: true,
    },
  },
})
