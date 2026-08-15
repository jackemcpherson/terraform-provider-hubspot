# 01: Freeze The API And Live Contract

Type: research
Status: claimed

## Scope

Revalidate current official documentation, execute the fingerprint-guarded live
probe, and freeze the v0.6.0 contract.

## Comments

- Official documentation was revalidated on 15 August 2026 with no blocking
  contradiction.
- The fresh live probe is pending because the local environment has no HubSpot
  access token, approved portal fingerprint, or protected Settings membership
  ID.
- The protected GitHub `northstar` environment has the access token and portal
  ID only. It has neither probe guard, and the retained ADR 0003 workflows do
  not provide a safe pre-merge execution path for the probe.
