# Security Policy

## Reporting a Vulnerability

Please report security vulnerabilities **privately** — do not open a public
issue, pull request, or discussion for anything security-sensitive.

The preferred channel is GitHub's private vulnerability reporting:

1. Go to the [Security tab](https://github.com/tdeschamps/modjo-cli/security)
   of this repository.
2. Click **Report a vulnerability** and fill in the advisory form.

This keeps the report confidential until a fix is available and a coordinated
disclosure can be made.

When reporting, please include:

- a description of the issue and its impact,
- the version (`modjo version`) and platform,
- steps to reproduce or a proof of concept, and
- any suggested remediation, if you have one.

We aim to acknowledge new reports within a few business days and to keep you
updated as we investigate and prepare a fix.

## Supported Versions

This project is pre-1.0. Security fixes are applied to the latest release; once
versioned releases are published, this section will list the supported range.

## Handling of Secrets

`modjo` stores credentials in the OS keychain when available (macOS Keychain,
Windows Credential Manager, libsecret/kwallet) and falls back to a `0600` file
otherwise. API keys are never written to the main config file and are only ever
displayed as masked fingerprints. If you discover a path where a secret is
logged, printed in full, or written world-readable, please treat it as a
security issue and report it through the channel above.
