// Engineering-only pages: they answer questions about the repository, not about
// DeskOS (docs/style.md, "Docs"). The site excludes them, and the sidebar
// coverage test uses the same list. A README.md at any level is an index for the
// repository and is excluded by the same rule.
const EXCLUDED = [
  'README.md',
  'style.md',
  'research-notes.md',
  'future-backlog.md',
  'images/PROVENANCE.md',
];

module.exports = { EXCLUDED };
