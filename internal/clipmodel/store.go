// Package clipmodel holds clipboard history and explicit content-enhancement
// drafts. It has no access to the system clipboard and never simulates pasting.
package clipmodel

import (
	"crypto/sha256"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrItemNotFound  = errors.New("clipboard item not found")
	ErrDraftNotFound = errors.New("enhancement draft not found")
	ErrPinLimit      = errors.New("pinned history limit reached")
	ErrInvalidPin    = errors.New("invalid pinned record")
)

// MaxPinnedItems bounds explicitly retained records independently of ordinary
// history. Retain additionally enforces the shared representation byte budget.
const MaxPinnedItems = 100

// Item is an immutable snapshot when returned from a Store. An empty AllowedApps
// slice means unrestricted; nonempty values are application bundle identifiers.
type Item struct {
	ID          string         `json:"id"`
	Type        ContentType    `json:"type"`
	Text        string         `json:"text"`
	Image       *ImageMetadata `json:"image,omitempty"`
	SourceApp   string         `json:"sourceApp"`
	SourceAppID string         `json:"sourceAppId,omitempty"`
	AllowedApps []string       `json:"allowedApps"`
	ParentID    string         `json:"parentId,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	Pinned      bool           `json:"pinned,omitempty"`
	fingerprint [sha256.Size]byte
}

// Allows reports whether the item permits use in appID. The native adapter is
// responsible for identifying the frontmost application and enforcing the rule.
// An unknown application is denied for restricted items.
func (item Item) Allows(appID string) bool {
	if len(item.AllowedApps) == 0 {
		return true
	}
	if appID == "" {
		return false
	}
	for _, allowed := range item.AllowedApps {
		if appID == allowed {
			return true
		}
	}
	return false
}

// Store keeps a bounded, newest-first history. Reading, searching and generating
// drafts never changes the current item. Public methods return independent
// snapshots, including independent AllowedApps slices and Image metadata.
type Store struct {
	mu        sync.RWMutex
	maxItems  int
	nextID    uint64
	items     []Item
	currentID string
	drafts    map[string]Draft
}

// NewStore creates an in-memory store. Capacity limits ordinary records; pins
// have their own limit. A nonpositive capacity uses 100 ordinary records.
func NewStore(maxItems int) *Store {
	if maxItems <= 0 {
		maxItems = 100
	}
	return &Store{maxItems: maxItems, drafts: make(map[string]Draft)}
}

// Capture records an external copy and makes it current without inheriting any
// earlier application's restrictions. An exact duplicate with the same policy
// refreshes its timestamp and moves to the front, keeping its ID and provenance.
// An ordinary copy never merges into a restricted record.
//
// ownWrite must only be true when the native adapter has identified an exact
// clipboard revision produced by its own write. Text equality is insufficient.
// Such an echo leaves history and current unchanged and returns recorded=false.
//
// An empty external value clears current without adding an empty history item.
// The adapter may also use it for unsupported clipboard content, so stale
// restrictions are not applied to new content.
func (s *Store) Capture(text, sourceApp string, ownWrite bool) (item Item, recorded bool) {
	return s.CaptureContent(Content{Text: text}, sourceApp, ownWrite)
}

// CaptureContent records supported text or PNG content and makes it current.
// recorded is true for both a new record and a duplicate refreshed in place;
// the adapter should associate the captured representations with the returned ID.
// Like Capture, a verified own-write echo changes nothing. Empty, invalid or
// unsupported external content clears current without adding history, so the
// previous item's restrictions cannot accidentally apply to a new clipboard.
func (s *Store) CaptureContent(content Content, sourceApp string, ownWrite bool) (item Item, recorded bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ownWrite {
		item, _ = s.currentLocked()
		return item, false
	}
	content, valid := normalizeContent(content)
	if !valid {
		s.currentID = ""
		return Item{}, false
	}
	return s.captureLocked(content, sourceApp, nil), true
}

// CaptureProtected records a completed protected-copy operation
// and makes it current atomically with its application restrictions already set.
// The native adapter should call it only after confirming the copied content and
// source application. For source-only copying, allowedApps contains that source
// application's bundle identifier.
//
// An empty text or an empty normalized policy is rejected without changing
// history or current; protected copying must never degrade into an unrestricted
// record. The policy is copied, trimmed and deduplicated. Exact copies merge
// only when their allowed-app sets match. A later ordinary copy cannot relax
// this record's restrictions, subject to normal history eviction.
func (s *Store) CaptureProtected(text, sourceApp string, allowedApps []string) (Item, bool) {
	return s.CaptureProtectedContent(Content{Text: text}, sourceApp, allowedApps)
}

// CaptureProtectedContent atomically captures supported text or PNG content
// with its restrictions. Invalid content or an empty normalized policy leaves
// current and history unchanged; protected copies never become unrestricted.
func (s *Store) CaptureProtectedContent(content Content, sourceApp string, allowedApps []string) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	apps := normalizeApps(allowedApps)
	content, valid := normalizeContent(content)
	if !valid || len(apps) == 0 {
		return Item{}, false
	}
	return s.captureLocked(content, sourceApp, apps), true
}

// Search performs case-insensitive text, content-type and source-app matching.
// Empty queries return all records in the same newest-first order as Items.
func (s *Store) Search(query string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]Item, 0, len(s.items))
	for _, item := range s.items {
		if query == "" ||
			strings.Contains(strings.ToLower(item.Text), query) ||
			strings.Contains(typeSearchTerms(item.Type), query) ||
			strings.Contains(strings.ToLower(item.SourceApp), query) {
			results = append(results, cloneItem(item))
		}
	}
	return results
}

// Select explicitly makes an existing item current, without reordering history.
// It returns the content metadata and policy for the native adapter to restore
// the corresponding clipboard representations. This method itself performs no
// operating-system operation.
func (s *Store) Select(id string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.ID == id {
			s.currentID = id
			return cloneItem(item), nil
		}
	}
	return Item{}, ErrItemNotFound
}

func (s *Store) Current() (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentLocked()
}

func (s *Store) Items() []Item {
	return s.Search("")
}

func (s *Store) Pinned() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []Item
	for _, item := range s.items {
		if item.Pinned {
			items = append(items, cloneItem(item))
		}
	}
	return items
}

// SetPinned changes retention only; it never selects or reorders a record.
// Pinned records have their own count limit and survive ordinary history churn.
func (s *Store) SetPinned(id string, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, count := -1, 0
	for i, item := range s.items {
		if item.ID == id {
			index = i
		}
		if item.Pinned {
			count++
		}
	}
	if index < 0 {
		return ErrItemNotFound
	}
	if pinned && !s.items[index].Pinned && count >= MaxPinnedItems {
		return ErrPinLimit
	}
	s.items[index].Pinned = pinned
	s.trimLocked()
	return nil
}

// RestorePinned restores metadata only after the adapter has validated all
// saved representations. It never makes a restored record current. IDs remain
// stable across launches, including enhancement lineage, and cannot collide
// with subsequent captures. Callers should validate a complete file into a new
// Store before installing that Store, avoiding partial loads.
func (s *Store) RestorePinned(item Item, content Content) error {
	item.Pinned = true
	return s.RestoreItem(item, content)
}

// RestoreItem installs a validated saved record without selecting it or restoring
// any authority over the current system clipboard. Callers validate a complete
// snapshot in a fresh Store before exposing it to the application.
func (s *Store) RestoreItem(item Item, content Content) error {
	content, valid := normalizeContent(content)
	number, err := strconv.ParseUint(strings.TrimPrefix(item.ID, "item-"), 10, 64)
	if !valid || !strings.HasPrefix(item.ID, "item-") || err != nil || number == 0 || number >= ^uint64(0)-1 ||
		item.CreatedAt.IsZero() || item.Type != content.Type || item.Text != content.Text ||
		(item.Image == nil) != (content.Image == nil) {
		return ErrInvalidPin
	}
	if item.Image != nil && *item.Image != *content.Image {
		return ErrInvalidPin
	}
	apps := normalizeApps(item.AllowedApps)
	if len(item.AllowedApps) != len(apps) {
		return ErrInvalidPin // A malformed policy must never become unrestricted.
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pins, ordinary := 0, 0
	for _, existing := range s.items {
		if existing.ID == item.ID {
			return ErrInvalidPin
		}
		if existing.Pinned {
			pins++
		} else {
			ordinary++
		}
	}
	if item.Pinned && pins >= MaxPinnedItems {
		return ErrPinLimit
	}
	if !item.Pinned && ordinary >= s.maxItems {
		return errors.New("ordinary history limit reached")
	}
	item.AllowedApps, item.fingerprint = apps, content.Fingerprint
	s.items = append(s.items, cloneItem(item))
	slices.SortStableFunc(s.items, func(a, b Item) int { return b.CreatedAt.Compare(a.CreatedAt) })
	s.nextID = max(s.nextID, number)
	return nil
}

// Remove evicts an item without changing the relative order of the survivors.
// Removing current clears it. Drafts remain available to cancel, but validation
// rejects a draft whose source was removed as ErrDraftStale.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, item := range s.items {
		if item.ID == id {
			copy(s.items[index:], s.items[index+1:])
			s.items[len(s.items)-1] = Item{}
			s.items = s.items[:len(s.items)-1]
			if s.currentID == id {
				s.currentID = ""
			}
			return nil
		}
	}
	return ErrItemNotFound
}

// SetAllowedApps changes a stored item's policy, including its current snapshot
// if selected. It copies, trims and deduplicates the caller's application IDs.
// Pending drafts retain their generation-time policy snapshot; Accept rejects
// them if it no longer matches the source policy.
func (s *Store) SetAllowedApps(id string, apps []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].AllowedApps = normalizeApps(apps)
			return nil
		}
	}
	return ErrItemNotFound
}

func (s *Store) currentLocked() (Item, bool) {
	for _, item := range s.items {
		if item.ID == s.currentID {
			return cloneItem(item), true
		}
	}
	return Item{}, false
}

func (s *Store) newIDLocked(prefix string) string {
	s.nextID++
	return prefix + "-" + strconv.FormatUint(s.nextID, 10)
}

func (s *Store) captureLocked(content Content, sourceApp string, allowedApps []string) Item {
	for index, existing := range s.items {
		if !sameCapturedContent(existing, content) || !sameAllowedApps(existing.AllowedApps, allowedApps) {
			continue
		}
		// Keep the first record's source and enhancement lineage. A new copy
		// from another app refreshes recency, not historical provenance.
		existing.CreatedAt = time.Now()
		copy(s.items[1:index+1], s.items[:index])
		s.items[0] = existing
		s.currentID = existing.ID
		return cloneItem(existing)
	}
	item := Item{
		ID:          s.newIDLocked("item"),
		Type:        content.Type,
		Text:        content.Text,
		Image:       content.Image,
		SourceApp:   sourceApp,
		SourceAppID: content.SourceAppID,
		AllowedApps: append([]string(nil), allowedApps...),
		CreatedAt:   time.Now(),
		fingerprint: content.Fingerprint,
	}
	s.insertLocked(item)
	return cloneItem(item)
}

func sameCapturedContent(item Item, content Content) bool {
	if content.Fingerprint == ([sha256.Size]byte{}) || item.fingerprint != content.Fingerprint ||
		item.Type != content.Type || item.Text != content.Text {
		return false
	}
	if item.Image == nil || content.Image == nil {
		return item.Image == nil && content.Image == nil
	}
	return *item.Image == *content.Image
}

func sameAllowedApps(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, app := range a {
		if !slices.Contains(b, app) {
			return false
		}
	}
	return true
}

func (s *Store) insertLocked(item Item) {
	s.items = append([]Item{cloneItem(item)}, s.items...)
	s.currentID = item.ID
	s.trimLocked()
}

func (s *Store) trimLocked() {
	ordinary, kept := 0, s.items[:0]
	for _, item := range s.items {
		if !item.Pinned {
			ordinary++
			if ordinary > s.maxItems {
				if s.currentID == item.ID {
					s.currentID = ""
				}
				continue
			}
		}
		kept = append(kept, item)
	}
	clear(s.items[len(kept):])
	s.items = kept
}

func cloneItem(item Item) Item {
	item.AllowedApps = append([]string(nil), item.AllowedApps...)
	if item.Image != nil {
		metadata := *item.Image
		item.Image = &metadata
	}
	return item
}

func normalizeApps(apps []string) []string {
	var result []string
	seen := make(map[string]bool, len(apps))
	for _, app := range apps {
		app = strings.TrimSpace(app)
		if app != "" && !seen[app] {
			result = append(result, app)
			seen[app] = true
		}
	}
	return result
}
