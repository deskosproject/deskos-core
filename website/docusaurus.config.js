// @ts-check
const { themes } = require('prism-react-renderer');
const { EXCLUDED } = require('./docs-pages.cjs');

/**
 * The site is a view of ../docs, not a second copy of it: Docusaurus reads the
 * same Markdown the repository reviews, and `sidebars.js` decides the reading
 * order. Nothing here is edited by hand on the site.
 *
 * @type {import('@docusaurus/types').Config}
 */
const config = {
  title: 'DeskOS',
  tagline: 'Workstations compiled from intent',
  favicon: 'img/logo.svg',

  // docs.deskos.org is its own GitHub Pages site, built and deployed from this
  // repository: the marketing site (deskos-site) is deliberately buildless and
  // points at the repository rather than duplicating the docs (website/README.md).
  url: 'https://docs.deskos.org',
  baseUrl: '/',
  organizationName: 'deskosproject',
  projectName: 'deskos-core',

  // A broken internal link is a build failure, not a warning: the site and the
  // repository must not drift apart because a page moved.
  onBrokenLinks: 'throw',
  // The pages are plain Markdown: `<name>` in a sentence is text, not JSX.
  markdown: {
    format: 'md',
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      {
        docs: {
          path: '../docs',
          routeBasePath: '/',
          sidebarPath: require.resolve('./sidebars.js'),
          // Engineering-only pages: they answer questions about the repository,
          // not about DeskOS (docs/style.md, "Docs").
          exclude: EXCLUDED,
          editUrl: 'https://github.com/deskosproject/deskos-core/edit/main/docs/',
          // A link that leaves docs/ is a repository link on the site. It runs
          // before the default plugins, which resolve Markdown links.
          beforeDefaultRemarkPlugins: [
            [
              require('./plugins/remark-repo-links'),
              {
                docsDir: require('node:path').resolve(__dirname, '../docs'),
                repoRoot: require('node:path').resolve(__dirname, '..'),
                excluded: EXCLUDED,
              },
            ],
          ],
        },
        blog: false,
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      },
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: 'img/logo.svg',
      navbar: {
        title: 'DeskOS',
        logo: {
          alt: 'DeskOS',
          src: 'img/logo.svg',
        },
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'docs',
            position: 'left',
            label: 'Documentation',
          },
          {
            href: 'https://github.com/deskosproject/deskos-core',
            label: 'GitHub',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Documentation',
            items: [
              { label: 'Introduction', to: '/' },
              { label: 'Quick start', to: '/quickstart' },
              { label: 'Installation', to: '/install' },
              { label: 'Platforms', to: '/platforms' },
            ],
          },
          {
            title: 'Project',
            items: [
              { label: 'Roadmap', to: '/roadmap' },
              { label: 'Decisions', to: '/category/decisions-adrs' },
              { label: 'Governance', to: '/governance' },
            ],
          },
          {
            title: 'Source',
            items: [
              {
                label: 'deskos-core',
                href: 'https://github.com/deskosproject/deskos-core',
              },
              {
                label: 'Issues',
                href: 'https://github.com/deskosproject/deskos-core/issues',
              },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} The DeskOS project. Licensed under Apache-2.0.`,
      },
      prism: {
        theme: themes.github,
        darkTheme: themes.dracula,
        additionalLanguages: ['bash', 'toml', 'json', 'yaml', 'ini'],
      },
      docs: {
        sidebar: {
          hideable: true,
        },
      },
    }),
};

module.exports = config;
