// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"encoding/json"
)

type PushChecker struct{}

func (PushChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	return Result{Status: StatusUnknown, ErrorMessage: "push check Run called directly; should not happen"}
}
