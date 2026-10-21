package nativeui

import (
	"slices"
	"time"
)

func (s *application) expireCopyFeedback() {
	if s.copyFeedback == nil {
		return
	}
	policy := s.native.CurrentPolicy()
	if s.native.Enabled() && s.native.ChangeCount() == s.copyFeedback.Version &&
		policy.Version == s.copyFeedback.Version &&
		slices.Equal(policy.AllowedApps, s.copyFeedback.AllowedApps) {
		return
	}
	s.copyFeedback = nil
	// Expire this exact copy's indication without clearing a newer action.
	if s.trayStatus.eventKind == "copy-protected" {
		s.trayStatus.reportEvent("", time.Now())
		s.message = ""
	}
	s.publish()
}
