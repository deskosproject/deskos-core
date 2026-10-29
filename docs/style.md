# Documentation style

Docs are read once, usually while something is broken. Every sentence has to pay
rent. These rules are enforced in review, and links are checked by
`python3 tools/check-links.py`, which `make check` runs.

## Rules

1. **One page, one question.** Answer it in the first two sentences.
2. **60–150 lines is the target.** 200 is the ceiling; past it, split the page or
   cut, and say which in the pull request.
3. **State the current fact.** How it got here belongs in an ADR or
   `research-notes.md`, not in a page a person reads to do a task.
4. **One fact, one place.** Link to it; never restate it. A second copy is a
   contradiction waiting to happen.
5. **Paragraphs of four lines or fewer.** Sentences of 25 words or fewer. Three
   or more items become a list.
6. **Tables for matrices, code fences for commands.** Bullet nesting stops at two.
7. **No filler.** Delete "it is worth noting", "in practice", "essentially",
   "robust", "powerful", "carefully", "we are proud". Delete any sentence that
   describes the document instead of the subject.
8. **Every claim carries its evidence**: a run id, a file, a test, a date. If
   there is no evidence, the sentence says it is unverified or it goes.
9. **Headings are sentence case, five words or fewer.** No numbering except
   ADRs. A heading someone links to does not move without updating the link.
10. **Delete rather than qualify.** A sentence that needs three qualifiers is
    usually a sentence that is not true yet.

## Page shapes

| Page | Sections |
|---|---|
| Reference (`resources/`, `checks.md`) | the table first, notes after |
| Procedure (`install.md`, `development.md`) | numbered steps, one command per fence |
| Design (`design/`) | what, why, what it rejects, what is unverified |
| ADR | Context (six lines at most), Decision, Consequences |

## Review checklist

- Does the first paragraph answer the page's question?
- Is there a paragraph longer than four lines, or a bullet nested three deep?
- Does any sentence repeat another page instead of linking to it?
- Does every claim name its evidence?
- If this page were deleted, what would be lost? If the answer is "nothing",
  merge it.

## Known debt

Pages over 200 lines are listed in the pull request that rewrites them; the
roadmap tracks what is left. Style debt is not an excuse to leave a contradiction
in place.
