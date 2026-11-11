package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

type Entry struct {
	UserID int64 `json:"user_id"`
	Up     int64 `json:"up_bytes"`
	Down   int64 `json:"down_bytes"`
}
type Report struct {
	NodeID      int64     `json:"node_id"`
	ReportID    string    `json:"report_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Entries     []Entry   `json:"entries"`
}
type Spool struct{ dir string }

func OpenSpool(dir string) (*Spool, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	if e := os.Chmod(dir, 0700); e != nil {
		return nil, e
	}
	return &Spool{dir}, nil
}

var batchID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

func (q *Spool) Enqueue(r Report) error {
	if r.NodeID <= 0 || !batchID.MatchString(r.ReportID) || len(r.Entries) != 1 || r.Entries[0].UserID <= 0 || r.Entries[0].Up < 0 || r.Entries[0].Down < 0 || r.Entries[0].Up > 1<<50 || r.Entries[0].Down > 1<<50 || !r.PeriodEnd.After(r.PeriodStart) || r.PeriodEnd.Sub(r.PeriodStart) > 24*time.Hour {
		return errors.New("invalid traffic batch")
	}
	pending, e := q.Pending()
	if e != nil {
		return e
	}
	if len(pending) >= 10000 {
		return errors.New("traffic queue full")
	}
	path := filepath.Join(q.dir, r.ReportID+".json")
	if _, e = os.Lstat(path); !os.IsNotExist(e) {
		return errors.New("batch already exists")
	}
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	return atomicWrite(path, b)
}
func (q *Spool) Pending() ([]string, error) {
	entries, e := os.ReadDir(q.dir)
	if e != nil {
		return nil, e
	}
	paths := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, errors.New("symlink in traffic queue")
			}
			paths = append(paths, filepath.Join(q.dir, entry.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}
func (q *Spool) Ack(path string) error {
	if filepath.Dir(path) != q.dir {
		return errors.New("batch outside spool")
	}
	if e := os.Remove(path); e != nil {
		return e
	}
	return syncDirectory(q.dir)
}
