// Package hooks is the policy extension surface for phi tool calls and session lifecycle.
//
// Configuration uses plugin.json under ~/.phi/hooks and
// <cwd>/.phi/hooks (see doc/hooks.md). PreToolUse runs before Gate; PostToolUse
// and PostToolUseFailure run after tool.Run; session events and Command slash
// hooks are supported as Phi extensions.
//
// [Manager] fans discovered command hooks across those events. [Discover] /
// [Load] build Managers from plugin.json. Set PHI_HOOKS=off to disable.
package hooks
