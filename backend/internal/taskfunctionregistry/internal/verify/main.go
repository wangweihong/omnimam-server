package main

import (
	"fmt"
	"os"

	"github.com/wangweihong/omnimam/backend/internal/taskfunctionregistry"
)

func main() {
	registry, err := taskfunctionregistry.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("validated %d ACTIVE function contracts (%s)\n", len(registry.ActiveFunctionRefs()), taskfunctionregistry.SourceMetadata())
}
