//go:build windows

package witness

import "os"

func terminationSignals() []os.Signal { return []os.Signal{os.Interrupt} }
