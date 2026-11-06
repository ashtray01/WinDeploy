package service

import (
	"errors"
	"strings"
	"testing"
)

func TestRuntimeErrorSanitizationAndJournalBound(t *testing.T) {
	s := New(&fixtureProvider{})
	defer s.ServiceShutdown()
	handler := RuntimeErrorHandler(s)
	handler(nil)
	if len(s.GetJournal()) != 1 {
		t.Fatal("invented error")
	}
	for i := 0; i < 220; i++ {
		handler(errors.New("private path and credentials"))
	}
	entries := s.GetJournal()
	if len(entries) != 200 || entries[0].ID >= entries[199].ID {
		t.Fatal("journal bound/order")
	}
	if entries[199].Level != "error" || strings.Contains(entries[199].Message, "private") {
		t.Fatal("raw diagnostic leaked")
	}
}
