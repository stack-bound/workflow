import { defineConfig } from 'vitepress'
import { readFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'

// Read the canonical version straight from the repo's VERSION file so the docs
// version badge always matches the binary without a second source of truth.
const version = readFileSync(
  fileURLToPath(new URL('../../VERSION', import.meta.url)),
  'utf-8',
).trim()

const repo = 'https://github.com/stack-bound/workflow'

export default defineConfig({
  lang: 'en-US',
  title: 'WorkFlow',
  titleTemplate: ':title · WorkFlow',
  description:
    'A local-first CLI that orchestrates git worktrees — one isolated workspace per piece of work — with a live-status dashboard and tmux integration.',

  // Cloudflare Pages serves the site at the domain root.
  base: '/',
  cleanUrls: true,
  lastUpdated: true,

  // Don't publish the capture tooling docs or the contributor README as pages.
  srcExclude: ['capture/**', 'README.md'],

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }],
    ['meta', { name: 'theme-color', content: '#10b981' }],
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'WorkFlow',

    nav: [
      { text: 'Guide', link: '/guide/introduction', activeMatch: '/guide/' },
      { text: 'Reference', link: '/reference/commands', activeMatch: '/reference/' },
      { text: 'Changelog', link: `${repo}/blob/master/CHANGELOG.md` },
      {
        text: `v${version}`,
        items: [
          { text: 'Changelog', link: `${repo}/blob/master/CHANGELOG.md` },
          { text: 'Releases', link: `${repo}/releases` },
        ],
      },
    ],

    sidebar: sidebar(),

    socialLinks: [{ icon: 'github', link: repo }],

    search: { provider: 'local' },

    editLink: {
      pattern: `${repo}/edit/master/docs/:path`,
      text: 'Edit this page on GitHub',
    },

    lastUpdated: { text: 'Last updated' },

    outline: { level: [2, 3], label: 'On this page' },

    docFooter: { prev: 'Previous', next: 'Next' },

    footer: {
      message: 'WorkFlow — orchestrate git worktrees from one cockpit.',
      copyright: 'Made by stack-bound · Built with VitePress',
    },
  },
})

function sidebar() {
  return [
    {
      text: 'Guide',
      collapsed: false,
      items: [
        { text: 'Introduction', link: '/guide/introduction' },
        { text: 'Installation', link: '/guide/installation' },
        { text: 'Getting Started', link: '/guide/getting-started' },
        { text: 'Core Concepts', link: '/guide/concepts' },
        { text: 'The Dashboard', link: '/guide/dashboard' },
        { text: 'tmux Integration', link: '/guide/tmux' },
        { text: 'Configuration', link: '/guide/configuration' },
        { text: 'Shell Integration', link: '/guide/shell-integration' },
        { text: 'Architecture & Design', link: '/guide/architecture' },
        { text: 'Troubleshooting', link: '/guide/troubleshooting' },
      ],
    },
    {
      text: 'Reference',
      collapsed: false,
      items: [
        { text: 'Command Reference', link: '/reference/commands' },
        { text: 'Configuration Reference', link: '/reference/configuration' },
      ],
    },
  ]
}
