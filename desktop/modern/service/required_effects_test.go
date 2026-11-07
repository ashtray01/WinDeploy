package service

import (
	"strings"
	"testing"
	"windeploy/core/shares"
)

func TestSharePartialOutcomeDoesNotClaimShortcutSuccess(t *testing.T) {
	view, err := shareOutcome(shares.Result{Connected: true, Warnings: []string{"Ярлык не подтверждён"}}, shares.ErrShortcutRequired, false)
	if err != nil || !view.Connected || view.ShortcutPath != "" || !strings.Contains(view.Message, "частично") || len(view.Warnings) == 0 {
		t.Fatalf("partial outcome lost: %+v %v", view, err)
	}
}
