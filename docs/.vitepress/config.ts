import { defineConfig } from 'vitepress'

// In dev the Studio is served separately on :3000; in prod it's bundled under /orca/studio/.
const playgroundLink = process.env.NODE_ENV === 'production' ? '/studio/' : 'http://localhost:3000'

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
  // Rewrite the __PLAYGROUND_URL__ placeholder in index.md frontmatter so the
  // hero Playground action points at the right URL for dev vs prod.
  transformPageData(pageData) {
    const actions = (pageData.frontmatter as any)?.hero?.actions
    if (Array.isArray(actions)) {
      for (const a of actions) {
        if (a?.link === '__PLAYGROUND_URL__') a.link = playgroundLink
      }
    }
  },
  title: 'Orca',
  description: 'A declarative language for defining AI agents',
  base: '/orca/',

  markdown: {
    lineNumbers: true,
    languageAlias: {
      'orca': 'hcl',
    },
  },

  head: [
    ['link', { rel: 'icon', href: '/orca/logo.png' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
  ],

  themeConfig: {
    nav: [
      { text: 'Docs', link: '/guide/introduction' },
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
      prev: false,
      next: false,
    },
  },
})
