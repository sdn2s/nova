package service

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const Name = "nova"

var Foreign = []string{"zapret", "GoodbyeDPI", "discordfix_zapret", "winws1", "winws2"}

var DriverServices = []string{"WinDivert", "WinDivert14"}

func Install(exe string, args []string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	if err := removeIfExists(m, Name); err != nil {
		return err
	}
	s, err := m.CreateService(Name, exe, mgr.Config{
		StartType:   mgr.StartAutomatic,
		DisplayName: "nova (winws)",
		Description: "DPI bypass: winws.exe managed by nova",
	}, args...)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()
	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	return waitState(s, svc.Running, 10*time.Second)
}

func Remove() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if err := removeIfExists(m, Name); err != nil {
		return err
	}
	for _, d := range DriverServices {
		if err := removeIfExists(m, d); err != nil {
			return err
		}
	}
	return nil
}

type Status struct {
	Installed bool
	State     string
	Command   string
	Foreign   []string
}

func Query() (Status, error) {
	var st Status
	m, err := mgr.Connect()
	if err != nil {
		return st, err
	}
	defer m.Disconnect()
	for _, n := range Foreign {
		if s, err := m.OpenService(n); err == nil {
			st.Foreign = append(st.Foreign, n)
			s.Close()
		}
	}
	s, err := m.OpenService(Name)
	if err != nil {
		return st, nil
	}
	defer s.Close()
	st.Installed = true
	if cfg, err := s.Config(); err == nil {
		st.Command = cfg.BinaryPathName
	}
	q, err := s.Query()
	if err != nil {
		return st, err
	}
	st.State = stateName(q.State)
	return st, nil
}

func State(name string) (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return "", nil
		}
		return "", err
	}
	defer s.Close()
	q, err := s.Query()
	if err != nil {
		return "", err
	}
	return stateName(q.State), nil
}

func Stop(name string) (bool, error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return false, nil
		}
		return false, err
	}
	defer s.Close()
	q, err := s.Query()
	if err != nil || q.State == svc.Stopped {
		return false, err
	}
	if _, err := s.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return true, err
	}
	return true, waitState(s, svc.Stopped, 10*time.Second)
}

func Start(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.Start(); err != nil {
		return err
	}
	return waitState(s, svc.Running, 10*time.Second)
}

func Delete(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	return removeIfExists(m, name)
}

func removeIfExists(m *mgr.Mgr, name string) error {
	s, err := m.OpenService(name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil
		}
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer s.Close()
	if q, err := s.Query(); err == nil && q.State != svc.Stopped {
		if _, err := s.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return fmt.Errorf("stop %s: %w", name, err)
		}
		if err := waitState(s, svc.Stopped, 10*time.Second); err != nil {
			return fmt.Errorf("stop %s: %w", name, err)
		}
	}
	if err := s.Delete(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
		return fmt.Errorf("delete %s: %w", name, err)
	}
	return nil
}

func waitState(s *mgr.Service, want svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		q, err := s.Query()
		if err != nil {
			return err
		}
		if q.State == want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout: state is %s, want %s", stateName(q.State), stateName(want))
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func stateName(s svc.State) string {
	switch s {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "starting"
	case svc.StopPending:
		return "stopping"
	case svc.Running:
		return "running"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("state %d", s)
	}
}
