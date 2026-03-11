// Package workspace defines the Workspace interface and registry for
// provisioning isolated agent execution environments.
//
// A Workspace represents an environment in which a harnessd agent loop runs:
// filesystem context, working directory, git repository, and a reachable
// harnessd HTTP endpoint. Workspaces are managed by an orchestration layer
// (e.g. symphd), not by the harness itself.
//
// Use the Registry to register and create workspace implementations by name:
//
//	workspace.Register("local", func() workspace.Workspace { return &LocalWorkspace{} })
//	ws, err := workspace.New(ctx, "local", workspace.Options{ID: "issue-42"})
package workspace
