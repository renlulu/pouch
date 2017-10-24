package ctrd

import (
	"context"
	"sync"
	"time"

	"github.com/containerd/containerd"
	"github.com/sirupsen/logrus"
)

// Message is used to watch containerd.
type Message struct {
	exitCode uint32
	exitTime time.Time
	err      error
}

// Error returns the error contained in Message.
func (m *Message) Error() error {
	return m.err
}

// HasError returns true if the error in message is not nil.
func (m *Message) HasError() bool {
	return m.err != nil
}

// ExitCode returns the exit code in Message.
func (m *Message) ExitCode() uint32 {
	return m.exitCode
}

// ExitTime returns the exit time in Message.
func (m *Message) ExitTime() time.Time {
	return m.exitTime
}

type watch struct {
	sync.Mutex
	client     *containerd.Client
	containers map[string]containerPack
}

func (w *watch) add(pack containerPack) {
	w.Lock()
	defer w.Unlock()

	w.containers[pack.id] = pack

	go func(pack containerPack) {
		status := <-pack.sch

		logrus.Infof("the task has quit, id: %s, err: %v, exitcode: %d, time: %v",
			pack.id, status.Error(), status.ExitCode(), status.ExitTime())

		if _, err := pack.task.Delete(context.Background()); err != nil {
			logrus.Errorf("failed to delete task, container id: %s", pack.id)
		}

		pack.ch <- &Message{
			err:      status.Error(),
			exitCode: status.ExitCode(),
			exitTime: status.ExitTime(),
		}

	}(pack)

	logrus.Infof("success to add container, id: %s", pack.id)
}

func (w *watch) remove(ctx context.Context, id string) error {
	w.Lock()
	defer w.Unlock()

	delete(w.containers, id)
	return nil
}

func (w *watch) get(id string) (containerPack, error) {
	w.Lock()
	defer w.Unlock()

	pack, ok := w.containers[id]
	if !ok {
		return pack, ErrContainerNotfound
	}
	return pack, nil
}

func (w *watch) notify(id string) chan *Message {
	w.Lock()
	defer w.Unlock()

	pack, ok := w.containers[id]
	if !ok {
		ch := make(chan *Message, 1)
		ch <- &Message{
			err: ErrContainerNotfound,
		}
		return ch
	}
	return pack.ch
}