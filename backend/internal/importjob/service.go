package importjob

import (
	"fmt"

	"github.com/okamyuji/go-react-state-machine-import/backend/internal/statemachine"
)

// Service HTTP層から呼び出されるImportJobのユースケースを定義するinterfaceです。
// ハンドラはこのinterfaceに依存するので、Repositoryやクロックの実装を差し替えても
// HTTP層には影響しません。
type Service interface {
	Create(filename string, rows int) (*Job, error)
	List() []*Job
	Get(id string) (*Job, error)
	Apply(id string, event statemachine.Event, processed int, message string) (*Job, error)
}

// service Serviceのデフォルト実装です。Repository / Clock / IDSourceという
// 3つのinterfaceのみに依存しているため、単体テストが非常に書きやすくなっています。
type service struct {
	repo    Repository
	machine *statemachine.Machine
	clock   Clock
	ids     IDSource
}

// NewService Serviceの実装をワイヤリングします。引数はすべてinterfaceなので、
// テストや別構成の運用環境で自由に差し替えできます。
func NewService(repo Repository, clock Clock, ids IDSource) Service {
	return &service{
		repo:    repo,
		machine: NewMachine(),
		clock:   clock,
		ids:     ids,
	}
}

// Create初期状態 (idle) の新しいジョブを登録します。
func (s *service) Create(filename string, rows int) (*Job, error) {
	if filename == "" {
		return nil, fmt.Errorf("importjob: filename は必須です")
	}
	if rows <= 0 {
		return nil, fmt.Errorf("importjob: rows は正の整数である必要があります")
	}
	now := s.clock.Now()
	job := &Job{
		ID:        s.ids.NextID(),
		Filename:  filename,
		Rows:      rows,
		State:     s.machine.Initial(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	job.View = ViewFor(job.State)
	if err := s.repo.Save(job); err != nil {
		return nil, err
	}
	return job.Clone(), nil
}

// List Repository任せです。並び順はRepositoryの責務としています。
func (s *service) List() []*Job { return s.repo.List() }

// Get Repositoryに委譲します。
func (s *service) Get(id string) (*Job, error) { return s.repo.FindByID(id) }

// Apply状態遷移を実行し、更新後のJobを永続化します。
// Repository.Saveが集約単位の置換なので、Repository側で遷移ロジックを
// 持つ必要はなく、責務がきれいに分離されます。
func (s *service) Apply(id string, event statemachine.Event, processed int, message string) (*Job, error) {
	job, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	next, err := s.machine.Next(job.State, event)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	job.Transitions = append(job.Transitions, TransitionEntry{
		From:  job.State,
		To:    next,
		Event: event,
		At:    now,
	})
	job.State = next
	job.View = ViewFor(next)
	job.UpdatedAt = now
	if processed >= 0 {
		job.Processed = processed
	}
	if message != "" {
		job.Message = message
	}
	if err := s.repo.Save(job); err != nil {
		return nil, err
	}
	return job.Clone(), nil
}
