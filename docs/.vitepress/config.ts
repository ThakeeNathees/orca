import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Orca',
  description: 'A declarative language for defining AI agents',
  base: '/orca/',

  head: [
    ['link', { rel: 'icon', href: '/orca/logo.png' }],
  ],

  themeConfig: {
    logo: {
      light: '/logo.png',
      dark: '/logo-dark.png',
    },

    nav: [
      { text: 'Guide', link: '/guide/introduction' },
      { text: 'Reference', link: '/reference/syntax-overview' },
      { text: 'Examples', link: '/examples/simple-agent' },
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
            { text: 'task', link: '/reference/task' },
            { text: 'knowledge', link: '/reference/knowledge' },
            { text: 'workflow', link: '/reference/workflow' },
            { text: 'trigger', link: '/reference/trigger' },
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
