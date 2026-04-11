package importjob

import (
	"errors"
	"sort"
	"sync"
)

// ErrNotFound指定されたIDのジョブが存在しないときに返されます。
var ErrNotFound = errors.New("インポートジョブが見つかりません")

// Repository ImportJob集約の永続化を抽象化するinterfaceです。
// Service層はこのinterfaceにのみ依存するので、インメモリ実装をSQL実装
// などに差し替えてもServiceやHTTPハンドラには一切手を入れずに済みます。
type Repository interface {
	Save(job *Job) error
	FindByID(id string) (*Job, error)
	List() []*Job
}

// InMemoryRepository sync.RWMutexで保護したmapを使う最小構成のRepository
// 実装です。ローカル開発とテストには十分で、標準ライブラリのみで書けます。
type InMemoryRepository struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewInMemoryRepository空のRepositoryを生成します。
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{jobs: make(map[string]*Job)}
}

// Saveジョブを保存または置換します。呼び出し側から渡されたJobのCloneを
// 保持することで外部からの変更を遮断します。
func (r *InMemoryRepository) Save(job *Job) error {
	if job == nil || job.ID == "" {
		return errors.New("importjob: Save には ID 付きの Job が必要です")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[job.ID] = job.Clone()
	return nil
}

// FindByID保持状態を守るためCloneを返します。
func (r *InMemoryRepository) FindByID(id string) (*Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return j.Clone(), nil
}

// List作成日時の新しい順にすべてのジョブを返します。安全のためCloneを返します。
func (r *InMemoryRepository) List() []*Job {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		out = append(out, j.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}
