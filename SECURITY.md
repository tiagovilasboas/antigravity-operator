# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.2.x   | :white_check_mark: |
| < 0.2.0 | :x:                |

## Reporting a Vulnerability

The Antigravity Operator team takes security seriously. If you discover a security vulnerability, please report it responsibly:

1. **Do NOT open a public issue.**
2. Send an email to `tcarvalhovb@gmail.com` with the subject line `[SECURITY] Potential Vulnerability in Antigravity Operator`.
3. Include:
   * A description of the vulnerability and its potential impact.
   * Steps to reproduce or proof-of-concept code.
   * Any suggested mitigations.

You will receive an acknowledgment within 48 hours, followed by updates on the assessment and timeline for a patch.

## Verifying Release Artifacts

Every release publishes `checksums.txt` (SHA-256 of each archive). `scripts/install.sh` refuses to install an archive that does not match it. Releases after v0.4.5 also carry a signed SLSA build provenance attestation made by `.github/workflows/release.yml`. To check an archive by hand:

```bash
sha256sum -c --ignore-missing checksums.txt
gh attestation verify agyo_linux_amd64.tar.gz --repo tiagovilasboas/antigravity-operator
```

All GitHub Actions used by the workflows are pinned to full commit SHAs, with the release tag in a comment.

## Privacy & Data Protection (LGPD & GDPR)

Antigravity Operator follows strict **Privacy by Design** principles:
- **Zero Telemetry:** No user data, code, or metrics are collected or transmitted.
- **Local Sovereignty:** All state is stored exclusively on your local machine.
- **Isolated Browser Profile:** Automation runs in an isolated user directory (`~/.gemini/antigravity-browser-profile`) without accessing personal credentials or browser history.

For our full compliance declaration with the Brazilian General Data Protection Law (**LGPD - Lei 13.709/2018**) and GDPR, see [PRIVACY.md](PRIVACY.md).
