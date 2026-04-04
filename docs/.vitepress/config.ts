import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Orca',
  description: 'A declarative language for defining AI agents',
  base: '/orca/',

  markdown: {
    languageAlias: {
      'orca': 'hcl',
    },
  },

  head: [
    ['link', { rel: 'icon', href: '/orca/logo.png' }],
  ],

  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/introduction' },
      { text: 'Reference', link: '/reference/syntax-overview' },
      { text: 'Examples', link: '/examples/simple-agent' },
      { text: 'Playground', link: '/studio/', target: '_self' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Guide',
          items: [
            { text: 'Introduction', link: '/guide/introduction' },
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'CLI Reference', link: '/guide/cli' },
          ],
        },
      ],
      '/reference/': [
        {
          text: 'Language Reference',
          items: [
            { text: 'Syntax Overview', link: '/reference/syntax-overview' },
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
      ],
      '/examples/': [
        {
          text: 'Examples',
          items: [
            { text: 'Simple Agent', link: '/examples/simple-agent' },
            { text: 'Multi-Agent Workflow', link: '/examples/multi-agent-workflow' },
          ],
        },
      ],
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
  },
})
