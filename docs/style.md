# Documentation style

Docs are read once, usually while something is broken. Every sentence has to earn
its place. `make style` checks the mechanical rules; the rest is review.

## Language

1. **Plain English.** Write "we checked", not "we evidenced"; "the weekly build",
   not "the gate"; "the records", not "the evidence" (except
   `evidence.deskos.org`). If a word sounds like a consultancy deck, replace it.
2. **No em dashes.** Use a comma, a colon, parentheses, or two sentences. The same
   goes for en dashes in prose; ranges keep their hyphen.
3. **No filler.** Delete "it is worth noting", "in practice", "essentially",
   "robust", "seamless", "comprehensive", "at its core", "not just X but Y".
4. **No meta-commentary.** A page describes its subject, not itself: no "this page
   explains", no "stated rather than implied", no repeating that a claim is
   honest. Say the fact; if there is no fact, cut the sentence.
5. **Active voice, short sentences.** Twenty-five words is the ceiling, not the
   target. A sentence with three qualifiers is usually not true yet: fix the fact
   or delete the sentence.
6. **Present tense, current fact.** History belongs in an ADR or in
   `research-notes.md`.

## Structure

1. **One page, one question.** Answer it in the first two sentences.
2. **60 to 150 lines.** 200 is the ceiling, and going past it needs a reason in
   the pull request.
3. **One fact, one place.** Link to it; never restate it. A second copy is a
   contradiction waiting to happen.
4. **Paragraphs of four lines or fewer.** Three or more items become a list.
5. **Tables for matrices, code fences for commands.** Bullet nesting stops at
   two. Every command a reader needs is complete and copy-pasteable.
6. **Headings in sentence case, five words or fewer.** No numbering except ADRs.
   A heading someone links to does not move without updating the link.

## Page shapes

| Page | Sections |
|---|---|
| Reference (`resources/`, `checks.md`) | the table first, notes after |
| Procedure (`install.md`, `development.md`) | numbered steps, one command per fence |
| Design (`design/`) | what, why, what it rejects, what is unverified |
| ADR | Context (six lines at most), Decision, Consequences |
| README | what it is, how to use it, what is proved, where to read more |

## Review checklist

- Does the first paragraph answer the page's question?
- Any em dash, buzzword or sentence about the document itself? Remove it.
- Does any sentence repeat another page instead of linking to it?
- Does every claim point at a run, a file or a test?
- If this page were deleted, what would be lost? If nothing, merge it.
