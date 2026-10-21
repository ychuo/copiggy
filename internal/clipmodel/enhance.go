package clipmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
)

type EnhancementKind string

const (
	CleanWhitespace EnhancementKind = "clean-whitespace"
	FormatJSON      EnhancementKind = "format-json"
)

var (
	ErrUnknownEnhancement = errors.New("unknown local enhancement")
	ErrUnsupportedContent = errors.New("enhancement requires text or JSON content")
	ErrDraftStale         = errors.New("draft source is unavailable or its allowed apps changed; regenerate the draft")
)

// Draft is a preview, not a history entry or the current clipboard. Its policy
// is copied from the source. Accept uses the internal snapshot identified by ID,
// so editing the returned value cannot silently remove its restrictions.
type Draft struct {
	ID          string          `json:"id"`
	Text        string          `json:"text"`
	SourceApp   string          `json:"sourceApp"`
	SourceAppID string          `json:"sourceAppId,omitempty"`
	AllowedApps []string        `json:"allowedApps"`
	ParentID    string          `json:"parentId"`
	Kind        EnhancementKind `json:"kind"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// Enhance runs a deterministic local transformation and stores a preview draft.
// It never changes the original record, history order or current selection.
// Failed and canceled transformations do not create a draft.
func (s *Store) Enhance(ctx context.Context, id string, kind EnhancementKind) (Draft, error) {
	if err := ctx.Err(); err != nil {
		return Draft{}, err
	}
	s.mu.RLock()
	var source Item
	found := false
	for _, item := range s.items {
		if item.ID == id {
			source = cloneItem(item)
			found = true
			break
		}
	}
	s.mu.RUnlock()
	if !found {
		return Draft{}, ErrItemNotFound
	}
	if source.Type != TypeText && source.Type != TypeJSON {
		return Draft{}, ErrUnsupportedContent
	}

	var text string
	switch kind {
	case CleanWhitespace:
		text = cleanWhitespace(source.Text)
	case FormatJSON:
		var buffer bytes.Buffer
		if err := json.Indent(&buffer, []byte(strings.TrimSpace(source.Text)), "", "  "); err != nil {
			return Draft{}, fmt.Errorf("format JSON: %w", err)
		}
		text = buffer.String()
	default:
		return Draft{}, ErrUnknownEnhancement
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Draft{}, err
	}
	draft := Draft{
		ID:          s.newIDLocked("draft"),
		Text:        text,
		SourceApp:   source.SourceApp,
		SourceAppID: source.SourceAppID,
		AllowedApps: append([]string(nil), source.AllowedApps...),
		ParentID:    source.ID,
		Kind:        kind,
		CreatedAt:   time.Now(),
	}
	s.drafts[draft.ID] = cloneDraft(draft)
	return cloneDraft(draft), nil
}

// ValidateDraft returns a trusted, independent preview snapshot without changing
// current or history. Native integration must validate before writing the system
// clipboard, then Accept only after that write succeeds. The adapter must
// serialize other store mutations across this validate/write/accept sequence;
// this method alone does not reserve or lock a draft between calls.
func (s *Store) ValidateDraft(draftID string) (Draft, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.validateDraftLocked(draftID)
}

func (s *Store) validateDraftLocked(draftID string) (Draft, error) {
	draft, ok := s.drafts[draftID]
	if !ok {
		return Draft{}, ErrDraftNotFound
	}
	sourceAvailable := false
	for _, source := range s.items {
		if source.ID == draft.ParentID {
			if !slices.Equal(source.AllowedApps, draft.AllowedApps) {
				return Draft{}, ErrDraftStale
			}
			sourceAvailable = true
			break
		}
	}
	if !sourceAvailable {
		return Draft{}, ErrDraftStale
	}
	return cloneDraft(draft), nil
}

// Accept consumes a draft, creates a new history record and makes it current.
// The original record is retained, subject to the normal history capacity. If
// the source policy has changed, or the source is no longer available to verify,
// it returns ErrDraftStale without changing current or history. Native callers
// should ValidateDraft, write that trusted value, then Accept as one serialized
// action, so a rejected draft is never written to the system clipboard first.
func (s *Store) Accept(draftID string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, err := s.validateDraftLocked(draftID)
	if err != nil {
		return Item{}, err
	}
	item := Item{
		ID:          s.newIDLocked("item"),
		Type:        classifyText(draft.Text),
		Text:        draft.Text,
		SourceApp:   draft.SourceApp,
		SourceAppID: draft.SourceAppID,
		AllowedApps: append([]string(nil), draft.AllowedApps...),
		ParentID:    draft.ParentID,
		CreatedAt:   time.Now(),
		fingerprint: textFingerprint(draft.Text),
	}
	s.insertLocked(item)
	delete(s.drafts, draftID)
	return cloneItem(item), nil
}

// Cancel discards a draft without changing history or current. It is safe to
// call more than once, including after a failed or canceled generation.
func (s *Store) Cancel(draftID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drafts, draftID)
}

func cloneDraft(draft Draft) Draft {
	draft.AllowedApps = append([]string(nil), draft.AllowedApps...)
	return draft
}

// cleanWhitespace normalizes line endings, trims trailing whitespace and
// collapses repeated blank lines. Indentation and spacing inside content lines
// remain intact, making this useful for both prose and copied code.
func cleanWhitespace(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if line == "" && (len(result) == 0 || result[len(result)-1] == "") {
			continue
		}
		result = append(result, line)
	}
	if len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return strings.Join(result, "\n")
}
