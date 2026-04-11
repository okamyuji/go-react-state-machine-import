package importjob

import (
	"strconv"
	"sync/atomic"
	"time"
)

// Clock 「現在時刻」を抽象化するinterfaceです。
// Service時刻を取得する以上の機能を必要としないので、interfaceはあえて1メソッドに絞っています。
type Clock interface {
	Now() time.Time
}

// IDSourceジョブIDを発行するinterfaceです。
// interfaceにすることで、テストでは決定論的なID生成器を差し込めます。
type IDSource interface {
	NextID() string
}

// SystemClock time.Nowを使う本番向けの実装です。
type SystemClock struct{}

// Now現在時刻を返します。
func (SystemClock) Now() time.Time { return time.Now() }

// SequentialID "prefix-1", "prefix-2" ... のように連番でIDを発行します。
// 並行アクセスに備えてatomicカウンタを使っています。
type SequentialID struct {
	prefix string
	seq    atomic.Int64
}

// NewSequentialID指定したプレフィックスで連番ID発行器を生成します。
func NewSequentialID(prefix string) *SequentialID {
	return &SequentialID{prefix: prefix}
}

// NextID次のIDを返します。
func (s *SequentialID) NextID() string {
	n := s.seq.Add(1)
	return s.prefix + "-" + strconv.FormatInt(n, 10)
}
