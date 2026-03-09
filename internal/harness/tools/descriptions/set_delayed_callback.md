Schedule a ONE-SHOT delayed callback that fires once after a specified delay. After the delay elapses, a new agent run starts on the CURRENT conversation with the given prompt.

USE THIS TOOL WHEN the user wants to:
- Check on something ONCE after a wait (e.g., "remind me in 10 minutes", "check the build in 5 minutes")
- Run a single follow-up task after a delay (e.g., "in 30 seconds, verify the deployment")
- Set a one-time timer for any future action

DO NOT USE THIS TOOL for recurring/repeating tasks. If the user says "every hour", "every 5 minutes", "daily", or any repeating schedule, use the cron tools instead.

Parameters:
- delay: Go duration string. Examples: "30s" (30 seconds), "5m" (5 minutes), "1h" (1 hour), "1h30m" (90 minutes). Minimum: 5s. Maximum: 1h.
- prompt: What the agent should do when the callback fires. Must be a meaningful instruction describing the task to perform.

The callback is in-process only and will be lost if the server restarts. Maximum 10 pending callbacks per conversation.