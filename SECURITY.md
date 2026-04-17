# Security Policy

## Supported Versions

Only the latest minor release line receives security updates.

| Version | Supported |
| ------- | --------- |
| 1.6.x   | ✅        |
| < 1.6   | ❌        |

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, report them privately via one of the following channels:

1. [GitHub Security Advisories](https://github.com/bmf-san/gogocoin/security/advisories/new) (preferred)
2. Email the maintainer listed in `go.mod` / repository profile

Please include as much of the following information as possible:

- Type of vulnerability (e.g. buffer overflow, SQL injection, authentication bypass, …)
- Affected version(s) and commit hash
- Step-by-step reproduction
- Potential impact
- Any suggested mitigation

## Response Process

- We aim to acknowledge receipt within **3 business days**.
- A triage decision (accept / decline / need more info) is targeted within **7 business days**.
- Confirmed vulnerabilities will receive a fix on `main` and, where applicable, a patch release on the supported minor line.
- A CVE will be requested via GitHub Security Advisories when the issue warrants one.

## Scope

In scope:
- The `gogocoin` Go library (`pkg/…`, `internal/…`)
- Example application (`example/…`) insofar as it demonstrates library misuse that leads to vulnerable deployments
- Provided Docker image (`example/Dockerfile`)

Out of scope:
- Third-party dependencies — please report upstream
- Configuration mistakes in downstream deployments that are not caused by insecure defaults in this repository
- Rate-limit / availability issues against external exchanges

## Safe Harbor

We support coordinated disclosure and will not pursue legal action against researchers who:

- Make a good-faith effort to follow this policy
- Avoid privacy violations, data destruction, service disruption, and any interference with live trading accounts
- Give us reasonable time to remediate before public disclosure
