# Domain Docs

## Before exploring

- Read root `CONTEXT.md` when present.
- Read ADRs under `docs/adr/` that affect the work.
- If either location is absent, proceed silently. Domain-modeling creates files lazily when terms or durable decisions crystallize.

## Layout

This is a single-context repository:

```text
/
├── CONTEXT.md
├── docs/adr/
└── internal/
```

## Vocabulary

Use canonical terms from `CONTEXT.md` in issues, designs, schemas, tests, and docs. Avoid synonyms explicitly rejected there. If a needed domain concept is missing, reconsider the term or raise the gap through domain-modeling.

## ADR conflicts

Surface conflicts with existing ADRs explicitly. Never silently override them.
