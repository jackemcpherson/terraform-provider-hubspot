# Preserve deferred paid and Enterprise capabilities

Type: task
Status: resolved
Blocked by: none

## Question

Before the v0.1 Free-only re-scope changes the provider surface, how will the
current pipeline, custom-schema, and sensitive-property implementation be
preserved on a named immutable Git ref, with an inventory sufficient to restore it
for a later release without making it part of v0.1's public contract?

## Answer

The paid and Enterprise baseline is preserved as annotated tag
`deferred/v0.2-paid-enterprise-baseline`, published to `origin` and dereferencing
to commit `f36e2b251ce2b0e93cd6b85bf4d9c9941701daa1`. Its tag object is
`e7b1ace66cba5cf5e14463487674ff63c97da455`.

[Deferred paid and Enterprise capabilities baseline](../deferred-capabilities-inventory.md)
records the registered surface, implementation, acceptance, workflow, and
consumer-documentation areas, plus the selective reintroduction rule. The later
Free-only work removes this surface from v0.1 registration and documentation; it
does not delete the preserved source or blindly merge it back later.
