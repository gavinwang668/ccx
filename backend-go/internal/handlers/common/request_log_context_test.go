package common

import "testing"

func TestRequestLogTagMasksSessionAndIncludesRound(t *testing.T) {
	tag := requestLogTag(RequestLogContext{
		SessionID: "c37c2ad4-1119-4eae-b603-db40668a489f",
		Round:     3,
	})

	want := "[session=c37c2ad4***489f round=3]"
	if tag != want {
		t.Fatalf("requestLogTag() = %q, want %q", tag, want)
	}
}

func TestTaggedFormatInsertsAfterComponentTag(t *testing.T) {
	got := taggedFormat("[Messages-Stream] 流式响应完成", "[session=abc round=2]")
	want := "[Messages-Stream] [session=abc round=2] 流式响应完成"
	if got != want {
		t.Fatalf("taggedFormat() = %q, want %q", got, want)
	}
}
