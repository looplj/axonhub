// 职责: 验证实时 smoke test 只接受已完成且附带用量的响应。
// 关联文件:
//   scripts/realtime-smoke/main.go

package main

import "testing"

func TestCheckCompletedResponse(t *testing.T) {
	for _, status := range []string{"completed", "failed", "incomplete", "cancelled", ""} {
		t.Run(status, func(t *testing.T) {
			event := map[string]any{"status": status, "usage": map[string]any{"total_tokens": 1}}
			if err := checkCompletedResponse(event); (err == nil) != (status == "completed") {
				t.Fatalf("status %q: unexpected validation result %v", status, err)
			}
		})
	}
	if err := checkCompletedResponse(map[string]any{"status": "completed"}); err == nil {
		t.Fatal("a completed response without usage must fail")
	}
}
