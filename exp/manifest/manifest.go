package manifest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/heatxsink/x/exp/storage"
)

var (
	manifestKey  = "manifest.json"
	versionCount = 5
)

type Manifest struct {
	startDate string
	baseURI   string
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

func createHash() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

func (m *Manifest) daysSince() int {
	start, _ := time.Parse("2006-01-02", m.startDate)
	elapsed := time.Since(start)
	return int(elapsed.Hours()) / 24
}

// New returns a Manifest rooted at baseURI. baseURI must be a storage URI
// that exp/storage understands (gs://bucket or file:///path).
func New(baseURI string, startDate string) *Manifest {
	return &Manifest{
		startDate: startDate,
		baseURI:   baseURI,
	}
}

func joinURI(base, key string) string {
	return strings.TrimSuffix(base, "/") + "/" + key
}

func (m *Manifest) Init(ctx context.Context) (*Item, []*Item, error) {
	ii, err := m.Load(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("loading manifest: %w", err)
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
	for _, obj := range objs {
		if matchesAny(obj.URI, fullPrefixes) {
			continue
		}
		fmt.Printf("-")
		if err := storage.Delete(ctx, obj.URI); err != nil {
			fmt.Println("storage.Delete(): ", err)
		}
	}
	fmt.Println()
	return nil
}

func getPrefixes(items []*Item, allowed []string) []string {
	ps := make([]string, 0, len(items)+len(allowed))
	for _, i := range items {
		ps = append(ps, i.Prefix)
	}
	ps = append(ps, allowed...)
	return ps
}

func matchesAny(uri string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(uri, p) {
			return true
		}
	}
	return false
}
