# 01: Freeze The API And Live Contract

Type: research
Status: resolved

## Scope

Revalidate current official documentation, execute the fingerprint-guarded live
probe, and freeze the v0.6.0 contract.

## Comments

- Official documentation was revalidated on 15 August 2026 with no blocking
  contradiction.
- The fresh live probe was initially reported as pending because the local
  environment had no HubSpot token, portal fingerprint, or Settings membership
  ID.
- The protected GitHub `northstar` environment had a token and portal ID only.
  It had neither probe guard nor a safe pre-merge execution path.
- The Keychain conclusion was corrected on 15 August 2026. The earlier check
  ran inside the filesystem sandbox, which hid the existing credential entry.
- The guarded live probe completed at `2026-08-15T04:16:43Z` with the existing
  Keychain credential and approved reusable global identity.

## Answer

The current official API and the guarded normal-Free lifecycle confirm the
frozen contract. The probe verified separate Settings and CRM identities, one
paginated join, bounded materialization, ordered profile writes, exact
restoration, welcome-disabled membership creation, and exact owned cleanup.
