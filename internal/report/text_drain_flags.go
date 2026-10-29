package report

import (
	"bytes"
	"fmt"
	"strings"
)

// writeDrainFlags writes the kubectl drain flags section. It writes nothing
// when no flag is missing and eviction is not disabled.
func writeDrainFlags(buf *bytes.Buffer, flags DrainFlags) {
	if len(flags.Missing) == 0 && !flags.EvictionDisabled {
		return
	}
	buf.WriteString("\n=== KUBECTL DRAIN FLAGS ===\n")
	if len(flags.Missing) > 0 {
		fmt.Fprintf(buf, "kubectl drain will refuse without: %s\n", strings.Join(flags.Missing, " "))
		buf.WriteString("(derived from the BLOCKED findings below, not a guarantee: run again with these flags to see what is left)\n")
	}
	if flags.EvictionDisabled {
		buf.WriteString("--disable-eviction is set: pods are deleted, not evicted, and PodDisruptionBudgets are not consulted.\n")
	}
}
