//go:build !windows

package hostprofile

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscoverRejectsUnreadableConfig(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can bypass file mode denial")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	_, err := Discover(context.Background(), Request{HostID: "cursor", Platform: runtime.GOOS, ExplicitConfigPath: path})
	assertHostError(t, err, "HOST_CONFIG_INACCESSIBLE")
}
