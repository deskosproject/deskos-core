# The documentation site

`deskos.org/docs` is [Docusaurus](https://docusaurus.io/) rendering the
repository's own `docs/` tree. The site is a **view**, not a second copy: there
is no page here that is edited here.

## How it is put together

| File | What it decides |
|---|---|
| `docusaurus.config.js` | the site's name, the `https://deskos.org/docs/` base URL, and which pages are excluded (`style.md`, `research-notes.md`, `future-backlog.md`, `docs/README.md`: they answer questions about the repository, not about DeskOS) |
| `sidebars.js` | the reading order: Introduction, Quick start, Installation, How-to guides, Concepts, Reference, Project. ADRs and design notes are read from their directories, so a new one appears without editing this file |
| `plugins/remark-repo-links.js` | a link that leaves `docs/` (`../tests/vm/README.md`, `../AGENTS.md`, an excluded page) becomes the same file on GitHub, so the site has no dead links and the Markdown keeps the form the repository's checker verifies |
| `src/css/custom.css` | the lockup and the accent; nothing that competes with the pages |

## Build it

```bash
npm ci --prefix website
npm run build --prefix website   # website/build
npm run start --prefix website   # live preview at http://localhost:3000/docs/
```

Node 20 or newer. `docs-site.yml` runs the same two commands on a pull request
that touches `docs/` or `website/`, and uploads `website/build` as the run's
artifact.

## Publish it

`docs.deskos.org` is a GitHub Pages site of this repository, deployed by
`docs-site.yml` from `main`. It is not a directory of `deskos.org`: the
marketing site (`deskosproject/deskos-site`) is deliberately buildless and says
docs belong in the repository, and putting a generated site inside it would make
that repo a build output.

Three things make it serve:

1. **Pages, once.** Repository settings → Pages → Source: *GitHub Actions*, and
   custom domain `docs.deskos.org`. The first deploy creates the
   `github-pages` environment the workflow names.
2. **The DNS record, once.** A DNS-only `CNAME docs → deskosproject.github.io`
   in `deskosproject/deskos-infrastructure` (`web.tf`). Proxying it hides the
   origin from the certificate authority and GitHub cannot issue the
   certificate, the same reason the apex records are DNS-only.
3. **The workflow.** A pull request builds the site; `main` deploys it with the
   repository's own token, so there is no second credential.

Nothing publishes from a branch, and a red build publishes nothing: the same
rule as the images. A preview of a pull request is its uploaded artifact, not a
staging site.

## Adding a page

Write it in `docs/` as any other page, then place its id in `sidebars.js`. A page
that is not in the sidebar still builds and is reachable by URL; it is simply not
part of the reading order.
