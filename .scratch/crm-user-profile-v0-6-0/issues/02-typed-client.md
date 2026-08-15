# 02: Implement The Typed CRM Profile Client

Type: task
Status: resolved

## Scope

Implement paginated discovery, exact reads, the unique Settings-ID join,
bounded readiness, identity checks, partial PATCH, and canonical working-hours
conversion.

## Comments

- Implementation proceeds against the frozen official contract and prior live
  evidence. Publication remains blocked by ticket 01.

## Answer

The typed client now provides paginated discovery, exact reads, a unique
Settings-ID join, bounded readiness, partial PATCH, identity checks, and
canonical working-hours parsing and serialization. Unit tests cover pagination,
timeouts, ambiguity, malformed responses, API rejection, and ordering.
