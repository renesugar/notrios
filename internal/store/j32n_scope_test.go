package store

import "testing"

// CollectionScopeSQLFor must mean exactly what CollectionScopeSQL means: an
// empty collection is every collection, a named one is that collection, and
// the parameter stays in the statement either way (v1.0 J32-N).
func TestCollectionScopeSQLForKeepsOneParameter(t *testing.T) {
	cases := []struct {
		alias        string
		collectionID string
		want         string
	}{
		{"", "default", "collection_id = ?"},
		{"d", "default", "d.collection_id = ?"},
		{"", "", "collection_id = COALESCE(NULLIF(?, ''), collection_id)"},
		{"d", "", "d.collection_id = COALESCE(NULLIF(?, ''), d.collection_id)"},
		{"", "   ", "collection_id = COALESCE(NULLIF(?, ''), collection_id)"},
	}
	for _, testCase := range cases {
		got := CollectionScopeSQLFor(testCase.alias, testCase.collectionID)
		if got != testCase.want {
			t.Errorf("alias %q collection %q: got %q, want %q",
				testCase.alias, testCase.collectionID, got, testCase.want)
		}
	}
}
