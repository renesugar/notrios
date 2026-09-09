package archivev2

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// spoolBuckets is the fan of temporary files a spool writes into. Bucket index
// is the first byte of a SHA-256 hex key, which is also the archive's first
// object-path segment, so iterating buckets in order and sorting each one
// yields globally sorted keys without ever holding them all in memory.
const spoolBuckets = 256

// spoolWriteBuffer bounds the per-bucket write buffer. 256 buckets at 16 KiB
// is 4 MiB of buffered output for the whole spool.
const spoolWriteBuffer = 16 << 10

// keySpool is a bounded external-sort spool for opaque line keys. Keys must
// not contain a newline; callers encode composite keys with a separator.
//
// It exists because both the archive writer and the archive verifier handle
// key sets proportional to the archive: an archive of a million-note library
// carries millions of object hashes and record identities, which cannot live
// on the Go heap. Peak memory is one sorted bucket, not the whole set.
type keySpool struct {
	root    string
	files   [spoolBuckets]*os.File
	writers [spoolBuckets]*bufio.Writer
	total   int64
}

func newKeySpool(root, name string) (*keySpool, error) {
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &keySpool{root: dir}, nil
}

// add appends one key. Buckets are opened lazily so a small archive creates a
// handful of files rather than 256.
func (s *keySpool) add(key string) error {
	if strings.ContainsAny(key, "\n\r") {
		return fmt.Errorf("spool keys cannot contain line breaks")
	}
	bucket := bucketFor(key)
	if s.writers[bucket] == nil {
		file, err := os.Create(filepath.Join(s.root, fmt.Sprintf("%02x", bucket)))
		if err != nil {
			return err
		}
		s.files[bucket] = file
		s.writers[bucket] = bufio.NewWriterSize(file, spoolWriteBuffer)
	}
	if _, err := s.writers[bucket].WriteString(key); err != nil {
		return err
	}
	if err := s.writers[bucket].WriteByte('\n'); err != nil {
		return err
	}
	s.total++
	return nil
}

func (s *keySpool) flush() error {
	for bucket, writer := range s.writers {
		if writer == nil {
			continue
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if err := s.files[bucket].Sync(); err != nil {
			return err
		}
	}
	return nil
}

func (s *keySpool) close() error {
	var firstErr error
	for bucket, writer := range s.writers {
		if writer == nil {
			continue
		}
		if err := writer.Flush(); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := s.files[bucket].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.writers[bucket] = nil
		s.files[bucket] = nil
	}
	if err := os.RemoveAll(s.root); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// sortedBucket reads one bucket and returns its keys in ascending order. The
// caller must not retain the slice across calls.
func (s *keySpool) sortedBucket(bucket int) ([]string, error) {
	path := filepath.Join(s.root, fmt.Sprintf("%02x", bucket))
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	keys := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	sort.Strings(keys)
	return keys, nil
}

// forEachSorted visits every key in globally ascending order.
func (s *keySpool) forEachSorted(visit func(key string) error) error {
	for bucket := 0; bucket < spoolBuckets; bucket++ {
		keys, err := s.sortedBucket(bucket)
		if err != nil {
			return err
		}
		for _, key := range keys {
			if err := visit(key); err != nil {
				return err
			}
		}
	}
	return nil
}

// bucketFor maps a key to its bucket. SHA-256 hex keys and prefixed record
// keys are both handled: the first two hex characters of a hash key select the
// bucket directly, and any other key is folded so distribution stays even.
func bucketFor(key string) int {
	if len(key) >= 2 {
		if high, ok := hexValue(key[0]); ok {
			if low, ok := hexValue(key[1]); ok {
				return high<<4 | low
			}
		}
	}
	hash := 0
	for index := 0; index < len(key); index++ {
		hash = (hash*31 + int(key[index])) & 0xffff
	}
	return hash & 0xff
}

func hexValue(character byte) (int, bool) {
	switch {
	case character >= '0' && character <= '9':
		return int(character - '0'), true
	case character >= 'a' && character <= 'f':
		return int(character-'a') + 10, true
	default:
		return 0, false
	}
}

// joinSpools merges a declaration spool and a reference spool bucket by bucket.
// It reports the first reference with no matching declaration and the first
// declaration carrying the given requireReferenced prefix that nothing
// referenced. Both directions are needed: a record may not name an object the
// archive omits, and the archive may not carry an object nothing names.
//
// Peak memory is one sorted bucket from each spool.
func joinSpools(declarations, references *keySpool, requireReferenced string) error {
	for bucket := 0; bucket < spoolBuckets; bucket++ {
		declared, err := declarations.sortedBucket(bucket)
		if err != nil {
			return err
		}
		required, err := references.sortedBucket(bucket)
		if err != nil {
			return err
		}
		if err := joinBucket(declared, required, requireReferenced); err != nil {
			return err
		}
	}
	return nil
}

func joinBucket(declared, required []string, requireReferenced string) error {
	declaredIndex, requiredIndex := 0, 0
	for declaredIndex < len(declared) || requiredIndex < len(required) {
		switch {
		case requiredIndex >= len(required):
			key := declared[declaredIndex]
			if err := checkUnreferenced(key, requireReferenced); err != nil {
				return err
			}
			if err := advanceUnique(declared, &declaredIndex, key); err != nil {
				return err
			}
		case declaredIndex >= len(declared):
			return missingDeclaration(required[requiredIndex])
		case declared[declaredIndex] == required[requiredIndex]:
			// Consume every repeat of this key on both sides.
			key := declared[declaredIndex]
			if err := advanceUnique(declared, &declaredIndex, key); err != nil {
				return err
			}
			for requiredIndex < len(required) && required[requiredIndex] == key {
				requiredIndex++
			}
		case declared[declaredIndex] < required[requiredIndex]:
			key := declared[declaredIndex]
			if err := checkUnreferenced(key, requireReferenced); err != nil {
				return err
			}
			if err := advanceUnique(declared, &declaredIndex, key); err != nil {
				return err
			}
		default:
			return missingDeclaration(required[requiredIndex])
		}
	}
	return nil
}

// advanceUnique moves past one declaration key and rejects a repeat, which is
// how duplicate record identities and duplicate object entries are detected.
func advanceUnique(declared []string, index *int, key string) error {
	*index++
	if *index < len(declared) && declared[*index] == key {
		return fmt.Errorf("duplicate archive declaration %q", key)
	}
	return nil
}

func checkUnreferenced(key, requireReferenced string) error {
	if requireReferenced != "" && strings.HasPrefix(key, requireReferenced) {
		return fmt.Errorf("unreferenced archive object %q", key)
	}
	return nil
}

func missingDeclaration(key string) error {
	return fmt.Errorf("archive record references missing or mismatched %q", key)
}
