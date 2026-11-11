package romm

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestMissingSyncScopes(t *testing.T) {
	tests := []struct {
		name string
		have []string
		want []string
	}{
		{
			name: "all present",
			have: []string{"assets.read", "assets.write", "devices.read", "devices.write", "extra"},
			want: nil,
		},
		{
			name: "missing device scopes",
			have: []string{"assets.read", "assets.write"},
			want: []string{"devices.read", "devices.write"},
		},
		{
			name: "empty",
			have: nil,
			want: []string{"assets.read", "assets.write", "devices.read", "devices.write"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MissingSyncScopes(tt.have); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MissingSyncScopes(%v) = %v, want %v", tt.have, got, tt.want)
			}
		})
	}
}

func TestValidateTokenRetrying_ASlowFirstAnswerIsNotABadToken(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			time.Sleep(300 * time.Millisecond)
		}
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, WithTimeout(100*time.Millisecond))
	err := client.ValidateTokenRetrying([]time.Duration{time.Millisecond}, IsTransient)

	if err != nil {
		t.Fatalf("a slow first answer failed the check: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestIsAuthRejection(t *testing.T) {
	for status, want := range map[int]bool{401: true, 403: true, 500: false, 404: false} {
		if got := IsAuthRejection(statusError(status, nil)); got != want {
			t.Errorf("status %d: IsAuthRejection = %v, want %v", status, got, want)
		}
	}
	if IsAuthRejection(ErrTimeout) {
		t.Error("a timeout was read as the token being refused")
	}
}
