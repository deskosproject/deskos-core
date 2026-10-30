// The reading order, not the file tree: the same Markdown is grouped the way a
// person reads it, and a page that moves between groups moves here and nowhere
// else. The ADRs and the design notes are read from the directory so a new one
// appears without editing this file.
const fs = require('node:fs');
const path = require('node:path');

const docsDir = path.resolve(__dirname, '../docs');

/**
 * Doc ids of one directory, sorted by file name, so the ADR number orders them.
 * Docusaurus strips a leading `001-` number prefix from the id, so the id keeps
 * the name only.
 */
function ids(dir) {
  return fs
    .readdirSync(path.join(docsDir, dir))
    .filter((file) => file.endsWith('.md') && file !== 'README.md')
    .sort()
    .map((file) => `${dir}/${file.replace(/^\d+-/, '').replace(/\.md$/, '')}`);
}

/** A category that opens on a generated index instead of a hand-written page. */
function group(label, description, items) {
  return {
    type: 'category',
    label,
    link: { type: 'generated-index', title: label, description },
    items,
  };
}

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docs: [
    { type: 'doc', id: 'mission', label: 'Introduction' },
    { type: 'doc', id: 'quickstart', label: 'Quick start' },
    { type: 'doc', id: 'install', label: 'Installation' },
    { type: 'doc', id: 'installer-iso', label: 'Installer ISO' },
    group(
      'How-to guides',
      'Tasks: build an image, update a machine, recover one, verify what you run.',
      [
        'development',
        'runtime',
        'recovery',
        'migrations',
        'troubleshooting',
        'diagnostics',
        'operations',
        'upstream',
      ],
    ),
    group(
      'Concepts',
      'What DeskOS is made of and why it is built this way.',
      [
        'architecture',
        'platforms',
        'fedora',
        'example-platforms',
        'resources',
        'gnome-coverage',
        'supply-chain',
        'compatibility',
        'threat-model',
        'support',
      ],
    ),
    group('Reference', 'The exact fields, keys and checks.', [
      'resources/core',
      'resources/desktop',
      'resources/software',
      'resources/system',
      'checks',
      'locks',
    ]),
    group('Project', 'Where DeskOS is going and how decisions are made.', [
      'roadmap',
      'governance',
      group('Decisions (ADRs)', 'One decision per record, with its context and consequences.', ids('adr')),
      group('Design notes', 'The longer investigations behind the decisions.', ids('design')),
    ]),
  ],
};

module.exports = sidebars;
