package clipboard

import (
	"context"
	"reflect"
	"testing"

	"copiggy/internal/clipmodel"
)

func TestQuotedFilePathTextCopiesTheExactOriginalWithScope(t *testing.T) {
	for _, raw := range []string{`"/tmp/a b.txt"`, `'~/Documents/a.txt'`, `"FiLe://LocalHost/tmp/a%20b.txt"`, `"/tmp/trailing space "`} {
		t.Run(raw, func(t *testing.T) {
			ctx := context.Background()
			history := NewHistory(nil, 0)
			original := clipmodel.TextData(raw)
			item, ok, err := history.Capture(ctx, original, clipmodel.Source{Name: "Fixture Editor"}, []string{"editor.app"})
			if err != nil || !ok || item.Type != clipmodel.TypeFilePath || item.Text != raw {
				t.Fatalf("quoted file capture: %+v, %v", item, err)
			}
			before := history.Records()
			writer := &fakeWriter{}
			ack, revision, err := (Copy{History: history, Port: writer}).Item(ctx, item.ID)
			if err != nil || revision != 42 || ack.ID != item.ID || ack.Text != raw || !reflect.DeepEqual(writer.data, original) ||
				!reflect.DeepEqual(writer.scope, []string{"editor.app"}) || !reflect.DeepEqual(before, history.Records()) {
				t.Fatalf("quoted path copy rewrote source or scope: %+v, %v", ack, err)
			}
		})
	}
}
