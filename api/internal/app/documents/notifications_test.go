package documents

import (
	"context"
	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
	"testing"
)

type captureActivity struct{ items []ports.ActivityNotice }

func (c *captureActivity) Activity(_ context.Context, n ports.ActivityNotice) {
	c.items = append(c.items, n)
}

func TestOnlyVisibleDocumentsNotifyClients(t *testing.T) {
	ctx := context.Background()
	events := &captureActivity{}
	svc := NewService(portstest.NewFakeDocumentRepository(), portstest.NewFakeMediaStore(), Options{Activity: events})
	d, err := svc.RecordUpload(ctx, "prac-1", ports.RecordUploadInput{Kind: document.KindSignedForm, ClientID: "client-1", PublicID: "terios/clients/client-1/signed_forms/a", Filename: "private.pdf", Bytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(events.items) != 0 {
		t.Fatal("private document leaked into feed")
	}
	yes := true
	_, err = svc.UpdateDocument(ctx, d.ID, document.Patch{VisibleToClient: &yes})
	if err != nil {
		t.Fatal(err)
	}
	if len(events.items) != 1 || events.items[0].ClientID != "client-1" || events.items[0].ClientLink != "/portal/documents" {
		t.Fatalf("missing share: %+v", events.items)
	}
	if err := svc.DeleteDocument(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if len(events.items) != 2 {
		t.Fatal("shared removal not notified")
	}
	_, err = svc.RecordUpload(ctx, "prac-1", ports.RecordUploadInput{})
	if err == nil || len(events.items) != 2 {
		t.Fatal("failed upload notified")
	}
}
