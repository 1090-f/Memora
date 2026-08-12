package preview

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/pkg/objectstore"
)

type memoryArtifactStore struct{ objects map[string][]byte }

func (m *memoryArtifactStore) PutObject(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	body, err := io.ReadAll(reader)
	if err == nil {
		m.objects[key] = body
	}
	return err
}

func (m *memoryArtifactStore) OpenObject(_ context.Context, key string) (io.ReadCloser, error) {
	body, ok := m.objects[key]
	if !ok {
		return nil, objectstore.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

func (m *memoryArtifactStore) StatObject(_ context.Context, key string) (*objectstore.ObjectInfo, error) {
	body, ok := m.objects[key]
	if !ok {
		return nil, objectstore.ErrObjectNotFound
	}
	return &objectstore.ObjectInfo{Key: key, Size: int64(len(body))}, nil
}

func (m *memoryArtifactStore) RemoveObject(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

func TestArtifactStorePublishesManifestLastAndValidatesPDF(t *testing.T) {
	storage := &memoryArtifactStore{objects: make(map[string][]byte)}
	artifacts := NewArtifactStore(storage, 1<<20)
	item := &entity.DocumentPreview{UserID: "u1", DocumentID: "d1", ContentVersion: 3, PreviewType: string(TypePDF), RenderHash: "render-hash", Renderer: "office", RendererVersion: "1.0"}
	pdf := []byte("%PDF-1.7\ncontent\n%%EOF")

	manifest, err := artifacts.Save(context.Background(), item, "source-hash", "strategy-v1", &RenderResult{Name: "rendered.pdf", MediaType: "application/pdf", Size: int64(len(pdf)), Reader: io.NopCloser(bytes.NewReader(pdf))})
	if err != nil {
		t.Fatal(err)
	}
	manifestKey := artifactPrefix(item) + "manifest.json"
	item.ManifestKey = &manifestKey
	item.ObjectKey = &manifest.Object.Key
	if _, ok := storage.objects[manifestKey]; !ok {
		t.Fatalf("manifest %q was not written", manifestKey)
	}
	file, err := artifacts.Open(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	file.Reader.Close()

	storage.objects[manifest.Object.Key] = []byte("NOT-A-PDF-BUT-SAME-SIZE!")[:len(pdf)]
	if _, err := artifacts.Open(context.Background(), item); !errors.Is(err, ErrArtifactCorrupt) {
		t.Fatalf("Open() error = %v, want ErrArtifactCorrupt", err)
	}
}

func TestArtifactStoreReportsMissingManifest(t *testing.T) {
	artifacts := NewArtifactStore(&memoryArtifactStore{objects: make(map[string][]byte)}, 1<<20)
	item := &entity.DocumentPreview{DocumentID: "d1", ContentVersion: 1, PreviewType: string(TypePDF), RenderHash: "hash"}
	if _, err := artifacts.LoadManifest(context.Background(), item); !errors.Is(err, ErrArtifactMissing) {
		t.Fatalf("LoadManifest() error = %v, want ErrArtifactMissing", err)
	}
}
