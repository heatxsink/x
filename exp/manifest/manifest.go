package manifest

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	io.WriteString(md5, time.Now().UTC().String())
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
		return nil, nil, fmt.Errorf("Load() %v", err)
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
	var ps []string
	for _, i := range items {
		ps = append(ps, i.Prefix)
	}
	for _, i := range allowed {
		ps = append(ps, i)
	}
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

func copyFile(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo
	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()
	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()
	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

func mkDir(path string) error {
	err := os.Mkdir(path, 0700)
	if err != nil {
		return err
	}
	return nil
}

func deployPath(ctx context.Context, bucket string, path string) error {
	files, err := getFiles(path)
	if err != nil {
		return err
	}
	for _, f := range files {
		k := strings.TrimPrefix(f, "deploy/")
		err := gcs.PutFile(ctx, bucket, k, f)
		if err != nil {
			return fmt.Errorf("gcs.PutFile: %v", err)
		}
		fmt.Printf("+")
	}
	fmt.Println()
	return nil
}

func getFiles(path string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return files, err
	}
	return files, nil
}
