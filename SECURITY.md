# Security Policy

## Supported Versions

Security fixes are prioritized for the current `main` branch and the latest published release. Older releases may receive fixes when the issue is severe and the patch can be applied safely.

## Reporting a Vulnerability

Please do not report security vulnerabilities in public issues, discussions, or pull requests.

Use GitHub private vulnerability reporting for this repository:

https://github.com/looplj/axonhub/security/advisories/new

If private vulnerability reporting is unavailable, open a public issue that only asks for a private security contact. Do not include exploit details, credentials, logs, tokens, or proof-of-concept payloads in the public issue.

## What to Include

Include enough information for maintainers to reproduce and assess the issue:

- Affected version, commit, or deployment mode.
- Vulnerability type and affected component.
- Reproduction steps or a minimal proof of concept.
- Expected impact and any required privileges.
- Relevant configuration details, with secrets removed.
- Whether the issue is already public or known to be exploited.

## Scope

Reports are especially useful when they involve:

- Authentication or authorization bypass.
- Exposure or misuse of provider API keys, OAuth tokens, or AxonHub API keys.
- Server-side request forgery or unsafe outbound requests.
- Cross-project or cross-user data access.
- Leakage of request traces, prompts, responses, logs, or usage data.
- Remote code execution, SQL injection, path traversal, or unsafe file access.
- Supply-chain risks in release artifacts, containers, or dependencies.

Out of scope:

- Reports that require physical access to a user's machine.
- Vulnerabilities only affecting unsupported or heavily modified deployments.
- Denial-of-service reports based solely on high-volume traffic without a specific flaw.
- Missing security headers on non-sensitive local development pages, unless they enable a concrete exploit.

## Coordinated Disclosure

After receiving a report, maintainers will try to:

- Acknowledge receipt within 7 days.
- Confirm the affected versions and severity when enough information is available.
- Prepare and release a fix before public disclosure when feasible.
- Credit reporters when requested and appropriate.

Please give maintainers a reasonable opportunity to fix the issue before publishing details.

## Safe Harbor

Good-faith security research is welcome when it:

- Avoids accessing, modifying, or deleting data that does not belong to you.
- Avoids service disruption, spam, social engineering, and physical attacks.
- Uses only the minimum testing needed to demonstrate the issue.
- Stops testing and reports promptly after confirming a vulnerability.

Research that follows these guidelines will be treated as authorized for the purpose of responsible disclosure.
