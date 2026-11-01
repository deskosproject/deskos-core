// Every Markdown page in docs/ is either in the sidebar or explicitly excluded.
//
// Docusaurus builds a page that no sidebar links to; it is reachable by URL but
// invisible to a reader. The site and the repository then drift one page at a
// time. This check makes the decision explicit: add the page to
// website/sidebars.js, or add it to EXCLUDED because it answers a question
// about the repository rather than about DeskOS.
const fs = require('node:fs');
const path = require('node:path');
const sidebars = require('../sidebars.js');
const { EXCLUDED } = require('../docs-pages.cjs');

const docsDir = path.resolve(__dirname, '../../docs');

function markdownFiles(dir, prefix = '') {
  const out = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const rel = prefix ? `${prefix}/${entry.name}` : entry.name;
    if (entry.isDirectory()) {
      out.push(...markdownFiles(path.join(dir, entry.name), rel));
    } else if (entry.isFile() && entry.name.endsWith('.md')) {
      out.push(rel);
    }
  }
  return out;
}

function docId(rel) {
  const parts = rel.replace(/\.md$/, '').split('/');
  // Docusaurus strips a leading `001-` number prefix from the file name.
  parts[parts.length - 1] = parts[parts.length - 1].replace(/^\d+-/, '');
  return parts.join('/');
}

function collect(items, out) {
  for (const item of items) {
    if (typeof item === 'string') {
      out.add(item);
    } else if (item.type === 'doc') {
      out.add(item.id);
    } else if (item.type === 'category') {
      collect(item.items, out);
    }
  }
}

const declared = new Set();
collect(sidebars.docs, declared);

const excluded = new Set();
for (const name of EXCLUDED) {
  excluded.add(name.replace(/\.md$/, ''));
}
for (const rel of markdownFiles(docsDir)) {
  if (path.basename(rel) === 'README.md') {
    excluded.add(docId(rel));
  }
}

const pages = markdownFiles(docsDir).map(docId);
const missing = pages.filter((id) => !declared.has(id) && !excluded.has(id));
const unknown = [...declared].filter((id) => !pages.includes(id));

if (missing.length || unknown.length) {
  if (missing.length) {
    console.error('pages in docs/ that are not in the sidebar and not excluded:');
    for (const id of missing) console.error(`  - ${id}`);
  }
  if (unknown.length) {
    console.error('sidebar entries with no page in docs/:');
    for (const id of unknown) console.error(`  - ${id}`);
  }
  console.error('Add the page to website/sidebars.js or to website/docs-pages.cjs.');
  process.exit(1);
}

console.log(`sidebar covers ${pages.length} docs page(s)`);
