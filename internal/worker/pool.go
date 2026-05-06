// Package worker предоставляет конфигурируемый пул горутин для асинхронной
// обработки задач. Применяется в транспортном слое: входящие CRDT-операции
// из WebSocket Hub (Fan-In) распределяются по N воркерам, каждый из которых
// применяет операцию к движку.
//
// Контракт graceful shutdown:
//   - Stop(ctx) ждёт окончания всех текущих задач или истечения ctx.
//   - Submit после Stop возвращает ErrPoolStopped.
package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Job — единица работы. Получает контекст пула: если Stop вызван,
// ctx будет отменён, и долгая задача может завершиться раньше.
type Job func(ctx context.Context)

// ErrPoolStopped возвращается из Submit после вызова Stop.
var ErrPoolStopped = errors.New("worker pool: stopped")

// Pool — пул воркеров фиксированного размера.
type Pool struct {
	size    int
	jobs    chan Job
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	stopped atomic.Bool
	once    sync.Once
}

// New создаёт пул с size воркерами и буферизованной очередью queueSize.
// size <= 0 → паника (некорректная конфигурация).
// queueSize <= 0 → синхронная очередь (unbuffered).
func New(size, queueSize int) *Pool {
	if size <= 0 {
		panic("worker.New: size must be > 0")
	}
	if queueSize < 0 {
		queueSize = 0
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		size:   size,
		jobs:   make(chan Job, queueSize),
		ctx:    ctx,
		cancel: cancel,
	}
	p.start()
	return p
}

func (p *Pool) start() {
	p.wg.Add(p.size)
	for i := 0; i < p.size; i++ {
		go p.worker()
	}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for job := range p.jobs {
		// Если контекст уже отменён — выполняем задачу всё равно,
		// но передаём отменённый ctx, чтобы она могла прерваться.
		job(p.ctx)
	}
}

// Submit ставит задачу в очередь. Блокируется, если очередь заполнена.
// Возвращает ErrPoolStopped, если пул остановлен.
func (p *Pool) Submit(job Job) error {
	if p.stopped.Load() {
		return ErrPoolStopped
	}
	select {
	case p.jobs <- job:
		return nil
	case <-p.ctx.Done():
		return ErrPoolStopped
	}
}

// TrySubmit — неблокирующая версия Submit. Возвращает false если очередь полна.
func (p *Pool) TrySubmit(job Job) (bool, error) {
	if p.stopped.Load() {
		return false, ErrPoolStopped
	}
	select {
	case p.jobs <- job:
		return true, nil
	default:
		return false, nil
	}
}

// Stop переводит пул в статус остановки и ждёт завершения всех задач,
// либо отмены ctx (что приведёт к отмене ctx задач). Идемпотентен.
//
// Поведение:
//  1. Закрывает входной канал — Submit больше не принимается.
//  2. Ждёт wg или ctx.Done(): если ctx истёк, дёргает cancel() пула,
//     чтобы текущие задачи получили сигнал отмены.
func (p *Pool) Stop(ctx context.Context) error {
	p.once.Do(func() {
		p.stopped.Store(true)
		close(p.jobs)
	})

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.cancel()
		return nil
	case <-ctx.Done():
		// Пробуждаем долгие задачи через отмену ctx пула.
		p.cancel()
		<-done
		return ctx.Err()
	}
}

// Size возвращает количество воркеров.
func (p *Pool) Size() int { return p.size }
