# Anthropic ToS Investigation: Subscription Use for Programmatic/Backend Services

**Date**: 2026-03-12
**Sources**: https://www.anthropic.com/legal/consumer-terms, https://www.anthropic.com/legal/aup, https://www.anthropic.com/legal/commercial-terms
**Question**: Does a Claude subscription (e.g., Claude Max at $200/month) permit programmatic/backend use via the `@anthropic-ai/claude-code` SDK to power a harness or backend service?

---

## 1. Short Answer

**Using a Claude consumer subscription (Claude Pro, Claude Max) to power a backend service or harness via the `@anthropic-ai/claude-code` SDK is prohibited by the Consumer Terms of Service.** The ToS explicitly bans automated/non-human access to Claude.ai except via an Anthropic API key. The intended path for programmatic and backend use is the **Anthropic API** (commercial terms), not a consumer subscription.

---

## 2. Key Clause: Automated Access Prohibition

From the **Consumer Terms of Service, Section 3 (Use of Our Services)**:

> "Except when you are accessing our Services via an Anthropic API Key or where we otherwise explicitly permit it, to access the Services through automated or non-human means, whether through a bot, script, or otherwise."

**Interpretation**: This is a prohibited use. Accessing Claude.ai (the service covered by consumer terms) via the Claude Code SDK — which calls Claude programmatically, not through a human-driven browser session — is automated/non-human access. The carve-out only applies if you are using an **Anthropic API Key**, which is governed by the Commercial Terms, not the Consumer Terms.

The Claude Code SDK (`@anthropic-ai/claude-code`) authenticates with your Claude.ai session credentials (OAuth), not an Anthropic API key. This means it falls under the Consumer Terms and the automated access prohibition applies.

---

## 3. Competing Products / Resale Prohibition

From the Consumer Terms, Section 3:

> "To develop any products or services that compete with our Services, including to develop or train any artificial intelligence or machine learning algorithms or models or resell the Services."

**Interpretation**: Using a consumer subscription to power a product or harness that you provide to others (even internally) could be characterized as "reselling the Services." The `go-agent-harness` is a backend service that wraps and re-exposes Claude capabilities to other users or systems — this is the pattern this clause targets.

---

## 4. Consumer Terms vs. Commercial Terms: The Critical Distinction

The Consumer Terms explicitly state:

> "Our Commercial Terms of Service govern your use of any Anthropic API key, the Anthropic Console, or any other Anthropic offerings that reference the Commercial Terms of Service. For clarity, this does not include Claude.ai or Claude Pro use for individuals or entities."

The Commercial Terms (for API access) explicitly permit what the Consumer Terms prohibit:

> "Companies may use the Services to power products and services Customer makes available to its own customers and end users."

**The split is clear**:
- **Consumer subscription (Claude Pro, Claude Max)**: Governed by Consumer Terms. Personal use only. No automated/programmatic access. No powering backends or products.
- **Anthropic API (paid per token or via enterprise contract)**: Governed by Commercial Terms. Explicitly permits building products and services on top of Claude.

---

## 5. What the AUP Says About Programmatic/Agentic Use

The Acceptable Use Policy applies universally to all Anthropic products. It does not add new restrictions on automated vs. manual use — that distinction is handled by the Consumer vs. Commercial Terms split. The AUP does note:

- "Agentic use cases must still comply with the Usage Policy."
- Model scraping and training on outputs is prohibited without authorization.
- High-risk use cases (legal, healthcare, financial, employment) require human-in-the-loop.

The AUP does **not** directly address the Consumer vs. API distinction, but it does not override the Consumer Terms prohibition on automated access.

---

## 6. Claude Code SDK Specifics

Neither the Consumer Terms, Commercial Terms, nor the AUP mention the `@anthropic-ai/claude-code` SDK by name. However, the SDK is relevant in the following way:

- The SDK exists to allow developers to programmatically invoke Claude Code (the agentic coding tool) as part of automated workflows.
- Claude Code itself is available on both the Claude Max subscription and via the API.
- **When used with a Claude Max subscription**, the SDK authenticates via session credentials — this is the consumer service accessed programmatically, which is prohibited.
- **When used with an Anthropic API key**, the SDK authenticates via API key — this falls under Commercial Terms and is permitted for building products.

Anthropic's own Claude Code documentation and the claude.com/product/claude-code page do not specify which subscription tier is required or what constitutes permitted programmatic use, but the legal documents make the boundary clear.

---

## 7. Assessment: Risk Level for go-agent-harness

The `go-agent-harness` use case — a backend HTTP service that runs LLM tool-calling loops and serves multiple clients via REST/SSE — is unambiguously the pattern the Consumer Terms prohibit:

1. **Automated access**: The harness invokes Claude programmatically via the SDK, not through human-driven browser interaction.
2. **Serving other users**: The harness exposes Claude capabilities to CLI clients, GUI frontends, and orchestration systems — effectively reselling/redistributing the service.
3. **Non-personal use**: A harness powering an agent orchestration platform is not "personal, non-commercial use."

**Risk**: Anthropic could suspend a consumer subscription account used this way. The Consumer Terms reserve the right to terminate access for ToS violations.

---

## 8. Compliant Path Forward

To use Claude programmatically for a backend service:

1. **Use the Anthropic API** directly (https://console.anthropic.com). This is governed by Commercial Terms which explicitly permit building products on top of Claude.
2. **Use an Anthropic API Key** in the harness configuration, not a consumer session credential.
3. The `@anthropic-ai/sdk` (the standard Anthropic API SDK, distinct from `@anthropic-ai/claude-code`) is the correct SDK for API-key-authenticated programmatic access.
4. Cost model changes: API pricing is per-token rather than a flat subscription. For heavy workloads, this may be more or less expensive than Claude Max depending on volume.

---

## 9. Open Questions / Grey Areas

- **Claude Max "extended agentic use"**: Anthropic markets Claude Max as enabling more agentic/automated tasks for individual developers. It is not entirely clear whether running Claude Code in headless/SDK mode for your own personal development workflows (not serving others) crosses the "automated access" line or is implicitly permitted by Anthropic's intent. The legal text is strict but the marketing language is permissive for agentic use. This warrants a direct question to Anthropic support if you want a definitive answer for personal/solo use.
- **Internal-only harness**: If the harness is used solely by the account holder (one person, not serving external users), the "reselling" concern is weaker, but the "automated access" prohibition in the Consumer Terms still technically applies.
- **Enterprise/Team plans**: Anthropic has Team and Enterprise plans. These may have different terms. Not investigated here.

---

## Sources Consulted

- Consumer Terms of Service: https://www.anthropic.com/legal/consumer-terms
- Acceptable Use Policy: https://www.anthropic.com/legal/aup
- Commercial Terms of Service: https://www.anthropic.com/legal/commercial-terms
- Claude Code product page: https://claude.com/product/claude-code
