package manifest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/heatxsink/x/exp/storage"
)

var (
	manifestKey  = "manifest.json"
	versionCount = 5
)

type Manifest struct {
	start   time.Time
	baseURI string
}

type Item struct {
	Published time.Time `json:"published"`
	Version   Version   `json:"version"`
	Prefix    string    `json:"prefix"`
}

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Point int `json:"point"`
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Point)
}

// hashCounter guarantees intra-process uniqueness in createHash even when
// two goroutines sample time.Now() at the same resolution tick.
var hashCounter atomic.Uint64

func createHash() string {
	h := sha256.New()
	_, _ = h.Write([]byte(time.Now().UTC().String()))
	_, _ = h.Write([]byte(strconv.FormatUint(hashCounter.Add(1), 10)))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

func (m *Manifest) daysSince() int {
	return int(time.Since(m.start).Hours()) / 24
}

// New returns a Manifest rooted at baseURI. baseURI must be a storage URI
// that exp/storage understands (gs://bucket, file:///path, or mem://ns).
// startDate must be a YYYY-MM-DD string; a parse failure returns an error
// instead of silently producing garbage Minor version numbers.
func New(baseURI string, startDate string) (*Manifest, error) {
	t, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("manifest: parse startDate %q: %w", startDate, err)
	}
	return &Manifest{start: t, baseURI: baseURI}, nil
}

func joinURI(base, key string) string {
	return strings.TrimSuffix(base, "/") + "/" + key
}

func (m *Manifest) Init(ctx context.Context) (*Item, []*Item, error) {
	ii, err := m.Load(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("loading manifest: %w", err)
	}
	if len(ii) == 0 {
		return nil, nil, fmt.Errorf("loading manifest: %w", storage.ErrNotExist)
	}
	oldMinor := ii[len(ii)-1].Version.Minor
	point := ii[len(ii)-1].Version.Point + 1
	minor := m.daysSince()
	if minor > oldMinor {
		point = 1
	}
	manifest := &Item{
		Published: time.Now(),
		Prefix:    createHash(),
		Version: Version{
			Major: 1,
			Minor: minor,
			Point: point,
		},
	}
	ii = append(ii, manifest)
	startIndex := len(ii) - versionCount
	if versionCount > len(ii) {
		startIndex = 0
	}
	endIndex := len(ii)
	ii = ii[startIndex:endIndex]
	return manifest, ii, nil
}

func (m *Manifest) Load(ctx context.Context) ([]*Item, error) {
	data, err := storage.Get(ctx, joinURI(m.baseURI, manifestKey))
	if err != nil {
		return nil, err
	}
	var items []*Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *Manifest) Save(ctx context.Context, items []*Item) error {
	data, err := json.MarshalIndent(items, "", "\t")
	if err != nil {
		return err
	}
	return storage.PutBytes(ctx, joinURI(m.baseURI, manifestKey), data, "application/json")
}

func (m *Manifest) Clean(ctx context.Context, items []*Item, allowed []string) error {
	objs, err := storage.List(ctx, m.baseURI)
	if err != nil {
		return err
	}
	prefixes := getPrefixes(items, allowed)
	fullPrefixes := make([]string, len(prefixes))
	for i, p := range prefixes {
		fullPrefixes[i] = joinURI(m.baseURI, p)
	}
	var errs []error
	for _, obj := range objs {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if matchesAny(obj.URI, fullPrefixes) {
			continue
		}
		if err := storage.Delete(ctx, obj.URI); err != nil {
			errs = append(errs, fmt.Errorf("delete %q: %w", obj.URI, err))
		}
	}
	return errors.Join(errs...)
}

func getPrefixes(items []*Item, allowed []string) []string {
	ps := make([]string, 0, len(items)+len(allowed))
	for _, i := range items {
		ps = append(ps, i.Prefix)
	}
	ps = append(ps, allowed...)
	return ps
}

// matchesAny reports whether uri equals one of the prefixes exactly
// (file-shaped allowed entries like "manifest.json") or sits under one as
// a directory prefix. The explicit "/" boundary avoids false matches
// between "KEEP" and "KEEPER"-style neighboring prefixes.
func matchesAny(uri string, prefixes []string) bool {
	for _, p := range prefixes {
		if uri == p || strings.HasPrefix(uri, p+"/") {
			return true
		}
	}
	return false
}
