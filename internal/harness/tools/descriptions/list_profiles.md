List all available agent profiles with their metadata.

Returns every profile visible to the harness across all three tiers (project, user, and built-in) in priority order. Each entry includes the profile name, description, configured model, allowed tools, and a source_tier field indicating where the profile was found ("project", "user", or "built-in").

Use this tool to discover which profiles are available before calling run_agent or get_profile.
