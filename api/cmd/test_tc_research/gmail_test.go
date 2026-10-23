package main

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"google.golang.org/api/gmail/v1"
)

func TestDecodeURLBase64(t *testing.T) {
	data := []byte{251, 255, 0, 1}
	for _, encoded := range []string{base64.RawURLEncoding.EncodeToString(data), base64.URLEncoding.EncodeToString(data)} {
		decoded, err := decodeURLBase64(encoded)
		if err != nil || string(decoded) != string(data) {
			t.Fatalf("decode failed: %v", err)
		}
	}
	for _, encoded := range []string{"!invalid", "a", "+/8="} {
		if _, err := decodeURLBase64(encoded); err == nil {
			t.Fatal("invalid URL-base64 accepted")
		}
	}
}

func TestPDFPartsIncludesRootAndNested(t *testing.T) {
	root := &gmail.MessagePart{Filename: originalFilename, Body: &gmail.MessagePartBody{}}
	nested := &gmail.MessagePart{Filename: originalFilename, Body: &gmail.MessagePartBody{}}
	root.Parts = []*gmail.MessagePart{nil, {Filename: originalFilename}, {Filename: editedFilename, Body: &gmail.MessagePartBody{}}, {Parts: []*gmail.MessagePart{nested}}}
	parts := pdfParts(root)
	if len(parts) != 2 || parts[0] != root || parts[1] != nested {
		t.Fatal("root/nested selection failed")
	}
	if len(pdfParts(nil)) != 0 {
		t.Fatal("nil root returned parts")
	}
}

func TestMatchingPDF(t *testing.T) {
	baseline := []byte("synthetic baseline")
	inline := func(data []byte) *gmail.MessagePart {
		return &gmail.MessagePart{Filename: originalFilename, Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString(data)}}
	}
	t.Run("inline mismatch then match stops before download", func(t *testing.T) {
		calls := 0
		root := &gmail.MessagePart{
			Headers: []*gmail.MessagePartHeader{nil, {Name: "fRoM", Value: "Research <sender@example.test>"}, {Name: "sUbJeCt", Value: "Actual confirmation subject"}},
			Parts:   []*gmail.MessagePart{inline([]byte("different")), inline(baseline), {Filename: originalFilename, Body: &gmail.MessagePartBody{AttachmentId: "unused"}}},
		}
		data, found, err := matchingPDF(context.Background(), root, "message", baseline, func(context.Context, string, string) (string, error) {
			calls++
			return "", errors.New("should not run")
		})
		if err != nil || !found || (data == nil || string(data.Data) != string(baseline)) || calls != 0 {
			t.Fatalf("match failed: %t, %v, calls=%d", found, err, calls)
		}
		if data.Metadata.From != "Research <sender@example.test>" || data.Metadata.Subject != "Actual confirmation subject" || data.Metadata.Filename != originalFilename {
			t.Fatal("actual fetched headers/attachment filename not preserved")
		}
	})
	t.Run("download padded data", func(t *testing.T) {
		root := &gmail.MessagePart{Filename: originalFilename, Body: &gmail.MessagePartBody{AttachmentId: "attachment"}}
		calls := 0
		_, found, err := matchingPDF(context.Background(), root, "message", baseline, func(_ context.Context, messageID, attachmentID string) (string, error) {
			calls++
			if messageID != "message" || attachmentID != "attachment" {
				t.Fatal("wrong attachment identity")
			}
			return base64.URLEncoding.EncodeToString(baseline), nil
		})
		if err != nil || !found || calls != 1 {
			t.Fatalf("download failed: %t, %v", found, err)
		}
	})
	t.Run("wrong hash not accepted", func(t *testing.T) {
		_, found, err := matchingPDF(context.Background(), inline([]byte("different")), "message", baseline, nil)
		if err != nil || found {
			t.Fatalf("wrong hash accepted: %v", err)
		}
	})
	t.Run("wrong filename not accepted", func(t *testing.T) {
		root := inline(baseline)
		root.Filename = editedFilename
		_, found, err := matchingPDF(context.Background(), root, "message", baseline, nil)
		if err != nil || found {
			t.Fatal("wrong filename accepted")
		}
	})
	t.Run("invalid data", func(t *testing.T) {
		root := inline(baseline)
		root.Body.Data = "invalid!"
		if _, _, err := matchingPDF(context.Background(), root, "message", baseline, nil); err == nil {
			t.Fatal("bad data accepted")
		}
	})
	t.Run("download error", func(t *testing.T) {
		root := &gmail.MessagePart{Filename: originalFilename, Body: &gmail.MessagePartBody{AttachmentId: "attachment"}}
		if _, _, err := matchingPDF(context.Background(), root, "message", baseline, func(context.Context, string, string) (string, error) {
			return "", errors.New("private server response")
		}); err == nil || err.Error() != "cannot download Gmail attachment" {
			t.Fatal("error not sanitized")
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := matchingPDF(ctx, inline(baseline), "message", baseline, nil); err == nil {
			t.Fatal("cancelled context accepted")
		}
	})
	t.Run("missing body data", func(t *testing.T) {
		root := &gmail.MessagePart{Filename: originalFilename, Body: &gmail.MessagePartBody{}}
		if _, found, err := matchingPDF(context.Background(), root, "message", baseline, nil); found || err != nil {
			t.Fatal("empty body accepted")
		}
	})
}
