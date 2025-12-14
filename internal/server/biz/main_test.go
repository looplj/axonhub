package biz

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	asyncReloadDisabled = true

	os.Exit(m.Run())
}
