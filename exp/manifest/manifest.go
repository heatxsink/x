package manifest

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/heatxsink/x/gcs"
)

var (
	manifestKey  = "manifest.json"
	versionCount = 5
)

type Manifest struct {
	startDate string
	bucket    string
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
	md5 := md5.New()
	_, _ = io.WriteString(md5, time.Now().UTC().String())
	return strings.ToUpper(fmt.Sprintf("%x", md5.Sum(nil)))
}

func (m *Manifest) daysSince() int {
	start, _ := time.Parse("2006-01-02", m.startDate)
	elapsed := time.Since(start)
	return int(elapsed.Hours()) / 24
}

func New(bucket string, startDate string) *Manifest {
	return &Manifest{
		startDate: startDate,
		bucket:    bucket,
	}
}

func (m *Manifest) Init(ctx context.Context) (*Item, []*Item, error) {
	ii, err := m.Load(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Load() %w", err)
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
	var items []*Item
	data, err := gcs.Get(ctx, m.bucket, manifestKey)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &items)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (m *Manifest) Save(ctx context.Context, items []*Item) error {
	data, err := json.MarshalIndent(items, "", "\t")
	if err != nil {
		return err
	}
	return gcs.PutBytes(ctx, m.bucket, manifestKey, data, "application/json")
}

func (m *Manifest) Clean(ctx context.Context, items []*Item, allowed []string) error {
	keys, err := gcs.List(ctx, m.bucket)
	if err != nil {
		return err
	}
	ps := getPrefixes(items, allowed)
	var saveThese []string
	for _, k := range keys {
		for _, p := range ps {
			if strings.HasPrefix(k.Name, p) {
				saveThese = append(saveThese, k.Name)
				break
			}
		}
	}
	for _, k := range keys {
		if !stringInSlice(k.Name, saveThese) {
			fmt.Printf("-")
			err := gcs.Delete(ctx, m.bucket, k.Name)
			if err != nil {
				fmt.Println("gcs.Delete(): ", err)
			}
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

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
