package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

var ErrNodeBlocked = errors.New("node has rejected reports requiring operator action")

func OpenSpool(dir string) (*Spool, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	if e := os.Chmod(dir, 0700); e != nil {
		return nil, e
	}
	if e:=syncDirectory(filepath.Dir(dir));e!=nil{return nil,e}
	return &Spool{dir}, nil
}

var batchID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

func (q *Spool) Enqueue(r Report) error {
	if r.NodeID <= 0 || !batchID.MatchString(r.ReportID) || len(r.Entries) != 1 || r.Entries[0].UserID <= 0 || r.Entries[0].Up < 0 || r.Entries[0].Down < 0 || r.Entries[0].Up > 1<<50 || r.Entries[0].Down > 1<<50 || !r.PeriodEnd.After(r.PeriodStart) || r.PeriodEnd.Sub(r.PeriodStart) > 24*time.Hour {
		return errors.New("invalid traffic batch")
	}
	if blocked, e := q.rejectedNode(r.NodeID); e != nil {
		return e
	} else if blocked {
		return ErrNodeBlocked
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
func (q *Spool) rejectedNode(id int64) (bool, error) {
	entries, e := os.ReadDir(filepath.Join(q.dir, "rejected", strconv.FormatInt(id, 10)))
	if os.IsNotExist(e) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			return true, nil
		}
	}
	return false, nil
}
func (q *Spool) RejectedNodes() (map[int64]bool, error) {
	out := map[int64]bool{}
	entries, e := os.ReadDir(filepath.Join(q.dir, "rejected"))
	if os.IsNotExist(e) {
		return out, nil
	}
	if e != nil {
		return nil, e
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id, e := strconv.ParseInt(entry.Name(), 10, 64)
		if e != nil || id <= 0 {
			continue
		}
		blocked, e := q.rejectedNode(id)
		if e != nil {
			return nil, e
		}
		if blocked {
			out[id] = true
		}
	}
	return out, nil
}

// Reject preserves the original immutable batch outside the deliverable queue.
// Only this node is paused; other nodes retain their independent delivery path.
func (q *Spool) Reject(path string, node int64, status int) error {
	if filepath.Dir(path) != q.dir || node < 0 {
		return errors.New("invalid rejected batch location")
	}
	dir := filepath.Join(q.dir, "rejected", strconv.FormatInt(node, 10))
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	if e:=syncDirectory(filepath.Dir(dir));e!=nil{return e}
	target := filepath.Join(dir, filepath.Base(path))
	if _, e := os.Lstat(target); !os.IsNotExist(e) {
		return errors.New("rejected batch already exists")
	}
	if e := os.Rename(path, target); e != nil {
		return e
	}
	if e := syncDirectory(dir); e != nil {
		return e
	}
	if e := syncDirectory(q.dir); e != nil {
		return e
	}
	reason, _ := json.Marshal(map[string]any{"node_id": node, "http_status": status, "rejected_at": time.Now().UTC()})
	return atomicWrite(target+".reason", reason)
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
