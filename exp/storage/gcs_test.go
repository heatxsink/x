package storage

import "testing"

func TestSplitGS(t *testing.T) {
	tests := []struct {
		uri        string
		bucket     string
		key        string
		wantErr    bool
	}{
		{"gs://bucket/key", "bucket", "key", false},
		{"gs://bucket/nested/key/path.json", "bucket", "nested/key/path.json", false},
		{"gs://bucket", "bucket", "", false},
		{"gs://bucket/", "bucket", "", false},
		{"gs://bucket/key%20with%20spaces", "bucket", "key with spaces", false},
		{"gs:///key", "", "", true},
		{"file:///foo", "", "", true},
		{"http://bucket/key", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.uri, func(t *testing.T) {
			b, k, err := splitGS(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("splitGS(%q) = (%q, %q), want error", tc.uri, b, k)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitGS(%q) unexpected error: %v", tc.uri, err)
			}
			if b != tc.bucket {
				t.Errorf("bucket = %q, want %q", b, tc.bucket)
			}
			if k != tc.key {
				t.Errorf("key = %q, want %q", k, tc.key)
			}
		})
	}
}
