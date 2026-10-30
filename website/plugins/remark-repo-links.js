// Links in the documentation point at the repository, because the repository is
// where those files are reviewed: `../tests/vm/README.md`, `../AGENTS.md`, an
// example root, or a page the site does not publish. On the site those targets
// do not exist, so this plugin rewrites them to the same file on GitHub and
// leaves every published page inside docs/ alone.
//
// The Markdown is not edited for the site: the repository keeps one link form
// that its own checker verifies, and the site resolves the rest.
const path = require('node:path');

const REPO = 'https://github.com/deskosproject/deskos-core/blob/main/';

function isExternal(url) {
  return /^(?:[a-z][a-z0-9+.-]*:|\/\/|#)/i.test(url);
}

/** Rewrite every link node whose target is not a published page. */
function walk(node, rewrite) {
  if (!node || typeof node !== 'object') return;
  if (node.type === 'link' && typeof node.url === 'string') rewrite(node);
  if (Array.isArray(node.children)) {
    for (const child of node.children) walk(child, rewrite);
  }
}

module.exports = function repoLinks({ docsDir, repoRoot, excluded = [] }) {
  const excludedPaths = new Set(
    excluded.map((file) => path.resolve(docsDir, file)),
  );

  return (tree, file) => {
    const here = path.dirname(file.path);
    walk(tree, (node) => {
      const url = node.url;
      if (isExternal(url)) return;
      const [target, hash] = url.split('#');
      if (!target) return;
      const absolute = path.resolve(here, target);
      const inDocs = absolute.startsWith(docsDir + path.sep);
      if (inDocs && !excludedPaths.has(absolute)) return;
      const relative = path.relative(repoRoot, absolute);
      node.url = REPO + relative + (hash ? `#${hash}` : '');
    });
  };
};
