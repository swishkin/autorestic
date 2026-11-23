package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/cupcakearmy/autorestic/internal/colors"
	"github.com/cupcakearmy/autorestic/internal/flags"
	"github.com/fatih/color"
)

func CheckIfCommandIsCallable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func CheckIfResticIsCallable() bool {
	return CheckIfCommandIsCallable(flags.RESTIC_BIN)
}

type ExecuteOptions struct {
	Command string
	Envs    map[string]string
	Dir     string
	Silent  bool
}

type ColoredWriter struct {
	target io.Writer
	color  *color.Color
}

func (w ColoredWriter) Write(p []byte) (n int, err error) {
	colored := []byte(w.color.Sprint(string(p)))
	w.target.Write(colored)
	return len(p), nil
}

type runningCommand struct {
	cmd  *exec.Cmd
	done chan struct{}
}

var (
	runningCommandsMutex sync.Mutex
	runningCommands      []*runningCommand
)

func registerRunningCommand(cmd *exec.Cmd) *runningCommand {
	running := &runningCommand{cmd: cmd, done: make(chan struct{})}
	runningCommandsMutex.Lock()
	runningCommands = append(runningCommands, running)
	runningCommandsMutex.Unlock()
	return running
}

func unregisterRunningCommand(running *runningCommand) {
	runningCommandsMutex.Lock()
	for i, other := range runningCommands {
		if other == running {
			runningCommands = append(runningCommands[:i], runningCommands[i+1:]...)
			break
		}
	}
	runningCommandsMutex.Unlock()
	close(running.done)
}

// Forward SIGTERM so restic can drop its lock on the way out, then force kill
func TerminateRunningCommands(timeout time.Duration) {
	runningCommandsMutex.Lock()
	running := make([]*runningCommand, len(runningCommands))
	copy(running, runningCommands)
	runningCommandsMutex.Unlock()

	for _, rc := range running {
		if rc.cmd.Process != nil {
			_ = rc.cmd.Process.Signal(syscall.SIGTERM)
		}
	}

	deadline := time.After(timeout)
	for _, rc := range running {
		select {
		case <-rc.done:
		case <-deadline:
			if rc.cmd.Process != nil {
				_ = rc.cmd.Process.Kill()
			}
			<-rc.done
		}
	}
}

func ExecuteCommand(options ExecuteOptions, args ...string) (int, string, error) {
	cmd := exec.Command(options.Command, args...)
	env := os.Environ()
	for k, v := range options.Envs {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env
	cmd.Dir = options.Dir

	if flags.VERBOSE {
		colors.Faint.Printf("> Executing: %s\n", cmd)
	}

	var out bytes.Buffer
	var error bytes.Buffer
	if flags.VERBOSE && !options.Silent {
		var colored ColoredWriter = ColoredWriter{
			target: os.Stdout,
			color:  colors.Faint,
		}
		mw := io.MultiWriter(colored, &out)
		cmd.Stdout = mw
	} else {
		cmd.Stdout = &out
	}
	cmd.Stderr = &error
	running := registerRunningCommand(cmd)
	err := cmd.Run()
	unregisterRunningCommand(running)
	if err != nil {
		code := -1
		if exitError, ok := err.(*exec.ExitError); ok {
			code = exitError.ExitCode()
		}
		return code, error.String(), err
	}
	return 0, out.String(), nil
}

func ExecuteResticCommand(options ExecuteOptions, args ...string) (int, string, error) {
	options.Command = flags.RESTIC_BIN
	var c = GetConfig()
	var optionsAsString = getOptions(c.Global, []string{"all"})
	args = append(optionsAsString, args...)
	return ExecuteCommand(options, args...)
}

func CopyFile(from, to string) error {
	original, err := os.Open(from)
	if err != nil {
		return nil
	}
	defer original.Close()

	new, err := os.Create(to)
	if err != nil {
		return nil
	}
	defer new.Close()

	if _, err := io.Copy(new, original); err != nil {
		return err
	}
	return nil
}

func CheckIfVolumeExists(volume string) bool {
	_, _, err := ExecuteCommand(ExecuteOptions{Command: "docker"}, "volume", "inspect", volume)
	return err == nil
}

func ArrayContains[T comparable](arr []T, needle T) bool {
	for _, item := range arr {
		if item == needle {
			return true
		}
	}
	return false
}
