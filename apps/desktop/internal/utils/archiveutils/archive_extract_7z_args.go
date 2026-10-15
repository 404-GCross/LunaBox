package archiveutils

import (
	"fmt"
	"runtime"
)

func build7zExtractArgs(source, target string) []string {
	return []string{
		"x",
		"-y",
		"-aoa",
		// Explicitly use all logical CPUs; individual codecs or solid blocks may still limit parallelism.
		fmt.Sprintf("-mmt=%d", runtime.NumCPU()),
		// Extraction progress is not consumed and only adds pipe output overhead.
		"-bd",
		"-o" + target,
		source,
	}
}
