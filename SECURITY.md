# Security policy

## Supported versions

| Version | Supported |
| --- | --- |
| 0.1.x | yes |
| < 0.1 | no |

## Reporting a vulnerability

Please report security issues privately to the maintainers of
`itzsepanta/RyxoUI` through GitHub's **Report a vulnerability** button on
the Security tab of the repository. Do not open a public issue and do not
mention the problem in a pull request.

Include:

- what the problem is and what an attacker gains,
- the version and the platform,
- the steps to reproduce, ideally a script,
- anything you already tried to rule out.

You will get an acknowledgement within a few days. After a fix is prepared
you will be asked whether the report may be published, and how you would
like to be credited.

## What the panel already does

- operator passwords are hashed with Argon2id (64 MiB, one pass);
- session tokens and API tokens are stored as SHA-256 hashes, never in
  clear;
- the session cookie is `HttpOnly`, `SameSite=Strict` and `Secure` whenever
  TLS is enabled;
- sign in attempts are throttled per account and recorded;
- every change is written to the audit log inside the same transaction as
  the change;
- every endpoint declares the scope it needs and fails closed;
- a release without a matching SHA-256 checksum is never installed by the
  self update capability;
- the panel refuses to send anything over plain HTTP when TLS is on, and
  never writes credentials to the log.

## Hardening advice for operators

- put the panel behind a reverse proxy that terminates TLS, and keep
  `web_tls_mode` consistent with it;
- give every operator the narrowest role that does the job;
- keep the database file on a volume that is not served to the web;
- review the audit log regularly;
- pin the panel version and update on your own schedule if the automatic
  check is not wanted.
