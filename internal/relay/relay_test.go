package relay

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name        string
		flagRelay   string
		flagDefault bool
		env         string
		want        string
		wantErr     bool
	}{
		{
			name: "all defaults uses built-in default",
			want: "croc.schollz.com:9009",
		},
		{
			name: "env overrides default",
			env:  "myrelay.example.com:9009",
			want: "myrelay.example.com:9009",
		},
		{
			name:        "flag-default overrides env",
			env:         "myrelay.example.com:9009",
			flagDefault: true,
			want:        "croc.schollz.com:9009",
		},
		{
			name:      "explicit flag wins over everything",
			env:       "myrelay.example.com:9009",
			flagRelay: "another.example.com:9009",
			want:      "another.example.com:9009",
		},
		{
			name:        "both flags is an error",
			flagRelay:   "foo:9009",
			flagDefault: true,
			wantErr:     true,
		},
		{
			name:      "host without port gets :9009 appended",
			flagRelay: "foo.example.com",
			want:      "foo.example.com:9009",
		},
		{
			name: "env without port gets :9009 appended",
			env:  "envrelay.example.com",
			want: "envrelay.example.com:9009",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.flagRelay, tt.flagDefault, tt.env)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}
