package cmd

import (
	"bytes"
	"encoding/base64"
	"testing"

	"google.golang.org/api/gmail/v1"
)

func TestCollectAttachments(t *testing.T) {
	p := &gmail.MessagePart{
		Parts: []*gmail.MessagePart{
			{
				Filename: "a.txt",
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{AttachmentId: "att1", Size: 123},
			},
			{
				Parts: []*gmail.MessagePart{
					{
						Filename: "b.pdf",
						MimeType: "application/pdf",
						Body:     &gmail.MessagePartBody{AttachmentId: "att2", Size: 456},
					},
				},
			},
		},
	}
	atts := collectAttachments(p)
	if len(atts) != 2 {
		t.Fatalf("unexpected: %#v", atts)
	}
	if atts[0].AttachmentID == "" || atts[1].AttachmentID == "" {
		t.Fatalf("missing attachment ids: %#v", atts)
	}
}

func TestBestBodyTextPrefersPlain(t *testing.T) {
	plain := base64.RawURLEncoding.EncodeToString([]byte("plain"))
	html := base64.RawURLEncoding.EncodeToString([]byte("<b>html</b>"))
	p := &gmail.MessagePart{
		Parts: []*gmail.MessagePart{
			{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: html}},
			{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: plain}},
		},
	}
	if got := bestBodyText(p); got != "plain" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestDecodeBase64URL(t *testing.T) {
	got, err := decodeBase64URL(base64.RawURLEncoding.EncodeToString([]byte("ok")))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "ok" {
		t.Fatalf("unexpected: %q", got)
	}
	got, err = decodeBase64URL(base64.URLEncoding.EncodeToString([]byte("ok")))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "ok" {
		t.Fatalf("unexpected: %q", got)
	}
	if _, err := decodeBase64URL("!!!"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDecodeBase64URLBytes(t *testing.T) {
	want := []byte{0xff, 0xff}

	got, err := decodeBase64URLBytes(base64.RawURLEncoding.EncodeToString(want))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected: %#v", got)
	}

	got, err = decodeBase64URLBytes(base64.URLEncoding.EncodeToString(want))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected: %#v", got)
	}

	enc := base64.RawURLEncoding.EncodeToString(want[:1])
	enc = enc[:1] + "\n" + enc[1:]
	got, err = decodeBase64URLBytes(enc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !bytes.Equal(got, want[:1]) {
		t.Fatalf("unexpected: %#v", got)
	}
}
