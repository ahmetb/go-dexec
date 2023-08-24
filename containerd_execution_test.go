package dexec

import (
	"context"
	"errors"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"regexp"
	"testing"
)

type container struct {
	mock.Mock
	containerd.Container
}

func (c *container) Task(ctx context.Context, attach cio.Attach) (containerd.Task, error) {
	args := c.Called(ctx, attach)
	err := args.Error(1)
	if taskIfc, ok := args.Get(0).(containerd.Task); ok {
		return taskIfc, err
	}
	return nil, err
}

func (c *container) Spec(ctx context.Context) (*oci.Spec, error) {
	args := c.Called(ctx)
	err := args.Error(1)
	if spec, ok := args.Get(0).(*oci.Spec); ok {
		return spec, err
	}
	return nil, err
}

func (c *container) Delete(ctx context.Context, opts ...containerd.DeleteOpts) error {
	inputArgs := make([]interface{}, 0, 1+len(opts))
	inputArgs = append(inputArgs, ctx)
	for _, opt := range opts {
		inputArgs = append(inputArgs, opt)
	}
	args := c.Called(inputArgs)
	return args.Error(0)
}

func (c *container) ID() string {
	return c.Called().String(0)
}

type task struct {
	mock.Mock
	containerd.Task
}

func (t *task) Exec(ctx context.Context, id string, spec *specs.Process, creator cio.Creator) (containerd.Process, error) {
	args := t.Called(ctx, id, spec, creator)
	err := args.Error(1)
	if ps, ok := args.Get(0).(containerd.Process); ok {
		return ps, err
	}
	return nil, err
}

func (t *task) Delete(ctx context.Context, opts ...containerd.ProcessDeleteOpts) (*containerd.ExitStatus, error) {
	inputArgs := make([]interface{}, 0, 1+len(opts))
	inputArgs = append(inputArgs, ctx)
	for _, o := range opts {
		inputArgs = append(inputArgs, o)
	}
	args := t.Called(inputArgs...)
	err := args.Error(1)
	if es, ok := args.Get(0).(*containerd.ExitStatus); ok {
		return es, err
	}
	return nil, err
}

type process struct {
	mock.Mock
	containerd.Process
}

func (p *process) Wait(ctx context.Context) (<-chan containerd.ExitStatus, error) {
	args := p.Called(ctx)
	err := args.Error(1)
	if ch, ok := args.Get(0).(<-chan containerd.ExitStatus); ok {
		return ch, err
	}
	return nil, err
}

func (p *process) Start(ctx context.Context) error {
	args := p.Called(ctx)
	return args.Error(0)
}

func Test_createTask_run(t *testing.T) {
	mockContainer := new(container)
	mockTask := new(task)
	spec := &oci.Spec{Process: &specs.Process{}}
	mockContainer.
		On("Task", mock.Anything, mock.Anything).Return(mockTask, nil).
		On("ID").Return("unit-test").
		On("Spec", mock.Anything).Return(spec, nil)

	mockPs := new(process)
	mockTask.On("Exec", mock.Anything, "unit-test-task", mock.Anything, mock.Anything).Return(mockPs, nil)

	ch := make(<-chan containerd.ExitStatus)
	mockPs.
		On("Wait", mock.Anything).Return(ch, nil).
		On("Start", mock.Anything).Return(nil)

	ct := &createTask{
		container: mockContainer,
	}
	_ = ct.run(Containerd{}, nil, io.Discard, io.Discard)

	mockContainer.AssertExpectations(t)
	mockTask.AssertExpectations(t)
	mockPs.AssertExpectations(t)
	assert.Equal(t, mockTask, ct.task)
	assert.Equal(t, mockPs, ct.process)
	assert.Equal(t, ch, ct.exitChan)
}

func Test_createTask_generateContainerName(t *testing.T) {
	ct := &createTask{
		opts: CreateTaskOptions{
			CommandDetails: CommandDetails{
				ExecutorId:      2,
				ChainExecutorId: 1,
				ResultId:        3,
			},
		},
	}
	expectedRegex := "chains-1-2-3-[a-zA-Z]{6}"
	containerId := ct.generateContainerName()
	assert.Regexp(t, regexp.MustCompile(expectedRegex), containerId)
}

func Test_createTask_createProcessSpec(t *testing.T) {
	mockContainer := new(container)
	ct := &createTask{
		container: mockContainer,
		cmd:       []string{"java", "-jar", "data-prep-cli.jar"},
		opts: CreateTaskOptions{
			User:       "61000",
			WorkingDir: "/go/src",
		},
	}

	spec := &oci.Spec{Process: &specs.Process{}}
	mockContainer.
		On("Spec", mock.Anything).
		Return(spec, nil)

	ps, _ := ct.createProcessSpec()
	assert.Equal(t, uint32(61000), ps.User.UID)
	assert.Equal(t, ct.opts.WorkingDir, ps.Cwd)
	assert.Equal(t, ps.Args, ct.cmd)
	mockContainer.AssertExpectations(t)
}

func Test_createTask_cleanup_NotFoundErrIgnoredOnTaskDelete(t *testing.T) {
	mockContainer := new(container)
	mockTask := new(task)
	ctx := context.Background()
	ct := &createTask{container: mockContainer, task: mockTask, ctx: ctx}

	mockTask.
		On("Delete", mock.Anything, mock.Anything).
		Return(nil, errdefs.ErrNotFound)

	mockContainer.
		On("Delete", mock.Anything, mock.Anything).
		Return(nil)

	err := ct.cleanup(Containerd{})
	assert.Nil(t, err)
	mockTask.AssertExpectations(t)
	mockContainer.AssertExpectations(t)
}

func Test_createTask_cleanup_ErrNotIgnored(t *testing.T) {
	mockContainer := new(container)
	mockTask := new(task)
	ctx := context.Background()
	ct := &createTask{container: mockContainer, task: mockTask, ctx: ctx}
	expectedErr := errors.New("unit test")
	mockTask.
		On("Delete", mock.Anything, mock.Anything).
		Return(nil, expectedErr)

	err := ct.cleanup(Containerd{})
	assert.ErrorIs(t, err, expectedErr)
	mockTask.AssertExpectations(t)
	mockContainer.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}
