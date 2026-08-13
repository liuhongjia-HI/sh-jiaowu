package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

const mutationTestDriverName = "starline-mutation-test"

var mutationDriverState = &mutationDBState{}

func init() { sql.Register(mutationTestDriverName, mutationTestDriver{}) }

type mutationDBState struct {
	mu         sync.Mutex
	failExec   bool
	statements []string
	commits    int
	rollbacks  int
}

func (s *mutationDBState) reset(failExec bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failExec = failExec
	s.statements = nil
	s.commits = 0
	s.rollbacks = 0
}

type mutationTestDriver struct{}

func (mutationTestDriver) Open(string) (driver.Conn, error) { return mutationTestConn{}, nil }

type mutationTestConn struct{}

func (mutationTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}
func (mutationTestConn) Close() error              { return nil }
func (mutationTestConn) Begin() (driver.Tx, error) { return mutationTestTx{}, nil }
func (mutationTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return mutationTestTx{}, nil
}
func (mutationTestConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	mutationDriverState.mu.Lock()
	defer mutationDriverState.mu.Unlock()
	mutationDriverState.statements = append(mutationDriverState.statements, query)
	if mutationDriverState.failExec {
		return nil, errors.New("injected database write failure")
	}
	return driver.RowsAffected(1), nil
}

type mutationTestTx struct{}

func (mutationTestTx) Commit() error {
	mutationDriverState.mu.Lock()
	defer mutationDriverState.mu.Unlock()
	mutationDriverState.commits++
	return nil
}
func (mutationTestTx) Rollback() error {
	mutationDriverState.mu.Lock()
	defer mutationDriverState.mu.Unlock()
	mutationDriverState.rollbacks++
	return nil
}

func TestPersistentMutationRollsBackWithoutPublishingMemory(t *testing.T) {
	mutationDriverState.reset(true)
	db, err := sql.Open(mutationTestDriverName, "")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()
	store := NewMemoryStore()
	store.db = db
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("load admin: %v", err)
	}
	before := len(store.Students(admin, learning.StudentQuery{}))
	_, err = store.CreateStudent("故障注入", admin, learning.StudentUpsertRequest{Name: "不应发布", Phone: "17793330000", Grade: "五年级"})
	if err == nil || !strings.Contains(err.Error(), "injected database write failure") {
		t.Fatalf("expected injected database error, got %v", err)
	}
	if got := len(store.Students(admin, learning.StudentQuery{})); got != before {
		t.Fatalf("memory advanced after rollback: got %d students, want %d", got, before)
	}
	mutationDriverState.mu.Lock()
	rollbacks := mutationDriverState.rollbacks
	mutationDriverState.mu.Unlock()
	if rollbacks != 1 {
		t.Fatalf("rollback count = %d, want 1", rollbacks)
	}
}

func TestPersistentMutationCommitsOnlyKeyedDelta(t *testing.T) {
	mutationDriverState.reset(false)
	db, err := sql.Open(mutationTestDriverName, "")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()
	store := NewMemoryStore()
	store.db = db
	admin, _ := store.PrincipalByUserID("user-super")
	before := len(store.Students(admin, learning.StudentQuery{}))
	if _, err := store.CreateStudent("局部事务", admin, learning.StudentUpsertRequest{Name: "已提交", Phone: "17793330001", Grade: "五年级"}); err != nil {
		t.Fatalf("create student: %v", err)
	}
	if got := len(store.Students(admin, learning.StudentQuery{})); got != before+1 {
		t.Fatalf("published student count = %d, want %d", got, before+1)
	}
	mutationDriverState.mu.Lock()
	statements := append([]string(nil), mutationDriverState.statements...)
	commits := mutationDriverState.commits
	mutationDriverState.mu.Unlock()
	if commits != 1 {
		t.Fatalf("commit count = %d, want 1", commits)
	}
	for _, statement := range statements {
		trimmed := strings.TrimSpace(strings.ToUpper(statement))
		if strings.HasPrefix(trimmed, "DELETE FROM") && !strings.Contains(trimmed, " WHERE ") {
			t.Fatalf("runtime mutation used unkeyed delete: %s", statement)
		}
	}
}

func TestPersistentMutationSerializesConcurrentDatabaseUpdates(t *testing.T) {
	mutationDriverState.reset(false)
	db, err := sql.Open(mutationTestDriverName, "")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()
	store := NewMemoryStore()
	store.db = db
	admin, _ := store.PrincipalByUserID("user-super")
	const writers = 8
	start := make(chan struct{})
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for index := 0; index < writers; index++ {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := store.CreateStudent("并发数据库事务", admin, learning.StudentUpsertRequest{
				Name: "并发事务学生", Phone: fmt.Sprintf("1779444%04d", index), Grade: "五年级",
			})
			if err != nil {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent persistent mutation: %v", err)
	}
	mutationDriverState.mu.Lock()
	commits := mutationDriverState.commits
	mutationDriverState.mu.Unlock()
	if commits != writers {
		t.Fatalf("commit count = %d, want %d", commits, writers)
	}
}

func TestDeltaRowsRoundTripThroughSameStateShape(t *testing.T) {
	store := NewMemoryStore()
	clone := store.cloneForMutation()
	builders := []func(*MemoryStore) []persistenceRow{identityRows, packageRows, contentRows, schedulingRows, commercialRows, engagementRows}
	for index, build := range builders {
		before := build(store)
		after := build(clone)
		sort.Slice(before, func(i, j int) bool { return before[i].key < before[j].key })
		sort.Slice(after, func(i, j int) bool { return after[i].key < after[j].key })
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("cloned restart state changed persistence rows for builder %d", index)
		}
	}
	if !reflect.DeepEqual(store.grants, clone.grants) || !reflect.DeepEqual(store.spaceAccess, clone.spaceAccess) {
		t.Fatal("cloned restart state changed grant persistence shape")
	}
}
