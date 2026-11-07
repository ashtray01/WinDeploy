package deploy

import (
	"context"
	"strings"
	"windeploy/core/engine"
	"windeploy/core/usercontext"
)

// Package catalogs do not yet declare user/machine install scope, so packages
// conservatively require the interactive user's context too.
func RequiresUserContext(task engine.Task) bool {
	switch task.Category {
	case "packages", "shares":
		return true
	case "browsers":
		return task.Parameters["scope"] != "machine" || strings.TrimSpace(task.Parameters["webapps_json"]) != ""
	case "windows":
		return strings.EqualFold(strings.TrimSpace(task.Parameters["setting"]), "explorer")
	case "printers":
		return parseBool(task.Parameters["set_default"])
	default:
		return false
	}
}

func checkUserContext(ctx context.Context, check func(context.Context) error) error {
	if check == nil {
		check = usercontext.Native{}.Check
	}
	return check(ctx)
}
