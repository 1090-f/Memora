package objectstore_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/platform/objectstore"
	"github.com/stretchr/testify/require"
)

func TestStoreRejectsInvalidObjectKeys(t *testing.T) {
	store := objectstore.New(fakeMinIOClient{})

	for _, key := range []string{"", `folder\\file.txt`, "folder/../file.txt"} {
		err := store.Put(context.Background(), key, strings.NewReader("x"), 1, "text/plain")
		require.ErrorIs(t, err, objectstore.ErrInvalidKey)
	}
}

type fakeMinIOClient struct{}

func (fakeMinIOClient) BucketExists(context.Context, string) (bool, error) { return true, nil }
func (fakeMinIOClient) PutObject(context.Context, string, string, io.Reader, int64, objectstore.PutOptions) error {
	return nil
}
func (fakeMinIOClient) GetObject(context.Context, string, string) (io.ReadCloser, error) {
	return nil, nil
}
func (fakeMinIOClient) RemoveObject(context.Context, string, string) error { return nil }
