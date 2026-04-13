# OpenClaw Research

**Date:** 2026-03-31

## What Is OpenClaw?

OpenClaw is a free, open-source autonomous AI agent that runs locally and connects large language models (LLMs) to real software on your machine. It uses messaging platforms (WhatsApp, Telegram, Slack, Discord, Signal, iMessage, Microsoft Teams, and many more) as its primary user interface. Unlike typical chatbots, OpenClaw can actually perform tasks on your computer -- manage files, send emails, run shell commands, browse the web, and control APIs.

## Origin and History

- **November 2025:** Originally published as **Clawdbot** by Austrian developer **Peter Steinberger**.
- **January 27, 2026:** Renamed to **Moltbot** following trademark complaints from Anthropic.
- **January 30, 2026:** Renamed again to **OpenClaw** (Steinberger found "Moltbot" didn't roll off the tongue).
- **February 14, 2026:** Steinberger announced he was joining OpenAI; the project was moved to an open-source foundation.
- **March 2, 2026:** 247,000 GitHub stars and 47,700 forks -- the fastest-growing open-source project on GitHub, surpassing React's 10-year record in 60 days.

## Architecture

OpenClaw is not an LLM itself; it is a **local orchestration layer** that gives existing models eyes, ears, and hands.

- **Gateway** (default port 18789): The control plane that routes incoming messages to the correct agent runtime, loads the right session, and passes it along.
- **Agent Runtime:** Loads relevant context from files and memory, compiles a system prompt, and sends it to the chosen LLM. When the model requests tool actions, the runtime executes them (shell commands, file operations, web browsing, etc.).
- **Memory:** Stores conversations and preferences for context continuity and autonomous workflows.

## Tools and Skills

OpenClaw distinguishes between **Tools** (capabilities) and **Skills** (knowledge of how to combine tools):

### Tools (what it *can* do)
- `read` / `write` -- file access
- `exec` -- run system commands
- `web_search` -- Google-like search
- `web_fetch` -- read web pages
- `browser` -- interact with pages (click, fill forms, screenshots)

### Skills (how it *knows* to do things)
- `gog` -- Google Workspace (email, calendar)
- `obsidian` -- note organization
- `github` -- repo management
- `slack` -- channel messaging
- Community-contributed skills via an `awesome-openclaw-skills` repository

## Lobster Workflow Engine

**Lobster** is a typed, local-first macro engine that transforms individual AI skills and tools into composable pipelines. Unlike autonomous agents that rely on LLMs to plan every step, Lobster executes multi-step tool sequences as a **single deterministic operation**, reducing latency and improving reliability.

## Multi-Agent / Collaborative Features

- **Multi-agent routing:** Route inbound channels/accounts/peers to isolated agents with separate workspaces and per-agent sessions.
- **Clawith** (clawith.ai): An open-source multi-agent collaboration platform built on OpenClaw. Agents have persistent identity, memory, and social networking -- they maintain focus items, create their own triggers (cron, polling, webhooks), manage their own schedules, and delegate tasks to each other like teammates onboarding into an organization.

## Security Concerns

- Cisco's AI security team found a third-party OpenClaw skill performing **data exfiltration and prompt injection** without user awareness.
- In March 2026, Chinese authorities restricted state-run enterprises and government agencies from running OpenClaw on office computers due to security risks.
- Cisco released **DefenseClaw**, a security layer for self-hosted OpenClaw deployments.

## Key Links

- GitHub: [github.com/openclaw/openclaw](https://github.com/openclaw/openclaw)
- Official site: [openclaw.ai](https://openclaw.ai/)
- Docs: [docs.openclaw.ai](https://docs.openclaw.ai/)
- Clawith (multi-agent): [clawith.ai](https://www.clawith.ai/)

## Sources

- [OpenClaw - Wikipedia](https://en.wikipedia.org/wiki/OpenClaw)
- [OpenClaw Explained - KDnuggets](https://www.kdnuggets.com/openclaw-explained-the-free-ai-agent-tool-going-viral-already-in-2026)
- [OpenClaw taking the tech world by storm - Yahoo Finance](https://finance.yahoo.com/news/openclaw-is-taking-the-tech-world-by-storm-heres-what-you-need-to-know-about-it-153752864.html)
- [Raise a lobster: OpenClaw in China - Fortune](https://fortune.com/2026/03/14/openclaw-china-ai-agent-boom-open-source-lobster-craze-minimax-qwen/)
- [Clawdbot to Moltbot to OpenClaw History - Blink Blog](https://blink.new/blog/clawdbot-moltbot-openclaw-history-2026)
- [DefenseClaw - Cisco Blogs](https://blogs.cisco.com/ai/cisco-announces-defenseclaw)
- [What Is OpenClaw - Clarifai](https://www.clarifai.com/blog/what-is-openclaw/)
- [OpenClaw Complete Tutorial - Towards AI](https://pub.towardsai.net/openclaw-complete-guide-setup-tutorial-2026-14dd1ae6d1c2)
- [What is OpenClaw - DigitalOcean](https://www.digitalocean.com/resources/articles/what-is-openclaw)
