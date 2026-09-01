package application

import (
	"context"
	"io"
	"sync"
)

// ResourceStream is a bounded, cancellation-aware reader over an attachment's
// bytes.
//
// Streaming rather than returning a []byte is the whole point: a resource can
// be gigabytes, the ABI must hand bytes across a C boundary in caller-sized
// chunks, and no adapter should be able to turn one attachment into one
// allocation. The limit is enforced here, so a caller that forgets to bound its
// own copy still cannot be made to read an unbounded amount.
type ResourceStream struct {
	ctx       context.Context
	source    io.ReadCloser
	remaining int64

	closeOnce sync.Once
	closeErr  error
}

// OpenResource opens an attachment's content for reading, refusing to serve
// more than maxBytes. The caller must Close the stream.
//
// The returned Resource is the metadata for the same attachment, so a caller
// can set a content type or length without a second lookup.
func (a *Application) OpenResource(ctx context.Context, id string, maxBytes int64) (Resource, *ResourceStream, error) {
	const op = "open_resource"
	if err := checkContext(ctx); err != nil {
		return Resource{}, nil, err
	}
	if err := requireID(op, "resource id", id); err != nil {
		return Resource{}, nil, err
	}
	if maxBytes <= 0 {
		return Resource{}, nil, invalid(op, "maxBytes must be positive")
	}
	found, reader, err := a.repo.OpenResourceContent(ctx, id)
	if err != nil {
		return Resource{}, nil, wrap(op, err)
	}
	if reader == nil {
		return Resource{}, nil, &Error{Kind: KindUnavailable, Op: op, Message: "resource bytes are not available"}
	}
	return resource(found), &ResourceStream{ctx: ctx, source: reader, remaining: maxBytes}, nil
}

// Read fills p with at most the remaining budget. It returns io.EOF once the
// budget is spent, so a caller cannot distinguish a truncated read from a
// complete one by error alone; compare what was read against Resource.SizeBytes
// when that distinction matters.
func (s *ResourceStream) Read(p []byte) (int, error) {
	if err := checkContext(s.ctx); err != nil {
		return 0, err
	}
	if s.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > s.remaining {
		p = p[:int(s.remaining)]
	}
	read, err := s.source.Read(p)
	s.remaining -= int64(read)
	if err == nil && s.remaining <= 0 {
		return read, io.EOF
	}
	return read, err
}

// Remaining reports how many bytes the stream will still serve.
func (s *ResourceStream) Remaining() int64 { return s.remaining }

// Close releases the underlying reader. It is safe to call more than once,
// which matters for the C ABI: a host that releases a handle twice must not
// close a reader twice.
func (s *ResourceStream) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = s.source.Close()
		s.remaining = 0
	})
	return s.closeErr
}
